package mtproto

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
	"github.com/gotd/td/transport"
	"golang.org/x/net/proxy"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
)

// SendMediaWidgetRequest implements raw MTProto serialization for messages.sendMedia#330e77f
// with InputMedia = inputMediaWidget#44f6c04e (Telegram Layer 233).
type SendMediaWidgetRequest struct {
	Peer     tg.InputPeerClass
	HTML     string
	Message  string
	RandomID int64
}

// Encode serializes messages.sendMedia with inputMediaWidget.
func (r *SendMediaWidgetRequest) Encode(b *bin.Buffer) error {
	b.PutID(0x330e77f) // messages.sendMedia#330e77f
	var flags bin.Fields
	if err := flags.Encode(b); err != nil {
		return err
	}
	if err := r.Peer.Encode(b); err != nil {
		return err
	}
	// media: inputMediaWidget#44f6c04e
	b.PutID(0x44f6c04e)
	var widgetFlags bin.Fields
	widgetFlags.Set(0) // allow_queries: true
	widgetFlags.Set(1) // fwd_allow_queries: true
	if err := widgetFlags.Encode(b); err != nil {
		return err
	}
	b.PutInt(0) // width = auto
	b.PutInt(0) // height = auto
	b.PutString(r.HTML)

	b.PutString(r.Message)
	b.PutLong(r.RandomID)
	return nil
}

// Sender describes MTProto widget sending capabilities.
type Sender interface {
	IsReady() bool
	SendWidget(ctx context.Context, chatID int64, html, caption string) error
}

// Client manages background MTProto connection to Telegram for sending native widgets.
type Client struct {
	log   *slog.Logger
	token string
	cfg   config.MTProtoConfig

	mu          sync.RWMutex
	tgCl        *telegram.Client
	ready       bool
	cancel      context.CancelFunc
	channelHash map[int64]int64
}

// NewClient initializes an MTProto client with bot credentials and MTProto configuration.
func NewClient(log *slog.Logger, token string, cfg config.MTProtoConfig) *Client {
	if cfg.AppID == 0 {
		cfg.AppID = 2040 // Default public Telegram Desktop API ID
	}
	if cfg.AppHash == "" {
		cfg.AppHash = "b18441a1ff607e10a989891a5462e627" // Default public Telegram Desktop API Hash
	}
	if cfg.SessionPath == "" {
		cfg.SessionPath = "data/mtproto.session"
	}
	return &Client{
		log:         log,
		token:       token,
		cfg:         cfg,
		channelHash: make(map[int64]int64),
	}
}

// Start launches MTProto connection in the background.
func (c *Client) Start(ctx context.Context) error {
	const op = "mtproto.Client.Start"

	if !c.cfg.Enabled {
		c.log.Info("MTProto client is disabled via configuration, using WebApp fallback", "operation", op)
		return nil
	}

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	readyCh := make(chan struct{})

	go func() {
		sessionDir := filepath.Dir(c.cfg.SessionPath)
		if sessionDir != "" && sessionDir != "." {
			if err := os.MkdirAll(sessionDir, 0o700); err != nil {
				c.log.Warn("failed to create MTProto session directory", "operation", op, "dir", sessionDir, "error", err)
			}
		}

		resolver, err := c.buildResolver()
		if err != nil {
			c.log.Error("failed to configure MTProto network resolver, disabling MTProto", "operation", op, "error", err)
			return
		}

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			client := telegram.NewClient(c.cfg.AppID, c.cfg.AppHash, telegram.Options{
				SessionStorage: &session.FileStorage{Path: c.cfg.SessionPath},
				Resolver:       resolver,
			})

			c.mu.Lock()
			c.tgCl = client
			c.mu.Unlock()

			err := client.Run(ctx, func(runCtx context.Context) error {
				status, sErr := client.Auth().Status(runCtx)
				if sErr != nil || !status.Authorized {
					authRes, authErr := client.Auth().Bot(runCtx, c.token)
					if authErr != nil {
						c.log.Warn("failed to authenticate bot via MTProto", "operation", op, "error", authErr)
						return authErr
					}
					user, _ := authRes.User.AsNotEmpty()
					c.log.Info("MTProto bot client connected and ready", "operation", op, "bot_id", user.ID, "username", user.Username)
				} else {
					user, _ := status.User.AsNotEmpty()
					c.log.Info("MTProto bot client connected from cached session", "operation", op, "bot_id", user.ID, "username", user.Username)
				}

				c.mu.Lock()
				c.ready = true
				c.mu.Unlock()

				select {
				case <-readyCh:
				default:
					close(readyCh)
				}

				<-runCtx.Done()
				return runCtx.Err()
			})

			c.mu.Lock()
			c.ready = false
			c.mu.Unlock()

			if ctx.Err() != nil {
				return
			}

			waitTime := 15 * time.Second
			if d, ok := tgerr.AsFloodWait(err); ok {
				waitTime = d + 2*time.Second
				c.log.Warn(fmt.Sprintf("MTProto flood wait active, pausing for %v", waitTime), "operation", op)
			} else if err != nil {
				c.log.Warn("MTProto client connection interrupted, retrying in 15s...", "operation", op, "error", err)
			}

			select {
			case <-ctx.Done():
				return
			case <-time.After(waitTime):
			}
		}
	}()

	// Wait up to 5 seconds for authentication
	select {
	case <-readyCh:
		return nil
	case <-time.After(5 * time.Second):
		c.log.Warn("MTProto client initialization timed out, continuing in background", "operation", op)
		return nil
	}
}

func (c *Client) buildResolver() (dcs.Resolver, error) {
	const op = "mtproto.Client.buildResolver"

	var dialFunc dcs.DialFunc
	if c.cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(c.cfg.ProxyURL)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid proxy url: %w", op, err)
		}
		dialer, err := proxy.FromURL(proxyURL, proxy.Direct)
		if err != nil {
			return nil, fmt.Errorf("%s: create proxy dialer: %w", op, err)
		}
		if ctxDialer, ok := dialer.(proxy.ContextDialer); ok {
			dialFunc = ctxDialer.DialContext
		} else {
			dialFunc = func(_ context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			}
		}
	}

	if c.cfg.MTProxyAddr != "" && c.cfg.MTProxySecret != "" {
		secretBytes, err := hex.DecodeString(c.cfg.MTProxySecret)
		if err != nil {
			return nil, fmt.Errorf("%s: decode mtproxy secret: %w", op, err)
		}
		return dcs.MTProxy(c.cfg.MTProxyAddr, secretBytes, dcs.MTProxyOptions{
			Dial: dialFunc,
		})
	}

	return dcs.Plain(dcs.PlainOptions{
		Dial:       dialFunc,
		Protocol:   transport.Abridged,
		Obfuscated: true,
	}), nil
}

// Stop shuts down the MTProto connection.
func (c *Client) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
}

// IsReady returns true if MTProto client is connected and authenticated.
func (c *Client) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready && c.tgCl != nil
}

// SendWidget sends a native messageMediaWidget to target chatID using MTProto.
func (c *Client) SendWidget(ctx context.Context, chatID int64, html, caption string) error {
	const op = "mtproto.Client.SendWidget"

	if !c.IsReady() {
		return fmt.Errorf("%s: MTProto client is not ready", op)
	}

	peer, err := c.resolvePeer(ctx, chatID)
	if err != nil {
		return fmt.Errorf("%s: resolve peer: %w", op, err)
	}

	var rndBuf [8]byte
	if _, err := crand.Read(rndBuf[:]); err != nil {
		return fmt.Errorf("%s: generate random id: %w", op, err)
	}
	rndID := int64(binary.BigEndian.Uint64(rndBuf[:]) & 0x7fffffffffffffff)

	req := &SendMediaWidgetRequest{
		Peer:     peer,
		HTML:     html,
		Message:  caption,
		RandomID: rndID,
	}

	var updates tg.UpdatesBox
	if err := c.tgCl.Invoke(ctx, req, &updates); err != nil {
		return fmt.Errorf("%s: invoke sendMedia: %w", op, err)
	}

	c.log.Info("native widget sent successfully via MTProto", "operation", op, "chat_id", chatID)
	return nil
}

func (c *Client) resolvePeer(ctx context.Context, chatID int64) (tg.InputPeerClass, error) {
	if chatID > 0 {
		return &tg.InputPeerUser{UserID: chatID, AccessHash: 0}, nil
	}

	// Telegram Bot API supergroup/channel IDs have -100 prefix: chatID <= -1000000000000
	if chatID <= -1000000000000 {
		channelID := -chatID - 1000000000000
		c.mu.RLock()
		hash, found := c.channelHash[channelID]
		c.mu.RUnlock()

		if found {
			return &tg.InputPeerChannel{ChannelID: channelID, AccessHash: hash}, nil
		}

		// Try resolving access hash from MTProto MessagesGetChats
		res, err := c.tgCl.API().MessagesGetChats(ctx, []int64{channelID})
		if err == nil {
			for _, chClass := range res.GetChats() {
				if ch, ok := chClass.(*tg.Channel); ok && ch.ID == channelID {
					if chHash, hasHash := ch.GetAccessHash(); hasHash {
						c.mu.Lock()
						c.channelHash[channelID] = chHash
						c.mu.Unlock()
						return &tg.InputPeerChannel{ChannelID: channelID, AccessHash: chHash}, nil
					}
				}
			}
		}

		return &tg.InputPeerChannel{ChannelID: channelID, AccessHash: 0}, nil
	}

	// Legacy basic group chat
	return &tg.InputPeerChat{ChatID: -chatID}, nil
}
