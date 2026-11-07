package mtproto

import (
	"context"
	crand "crypto/rand"
	"encoding/binary"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"
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
	log     *slog.Logger
	token   string
	appID   int
	appHash string

	mu          sync.RWMutex
	tgCl        *telegram.Client
	ready       bool
	cancel      context.CancelFunc
	channelHash map[int64]int64
}

// NewClient initializes an MTProto client with bot credentials.
func NewClient(log *slog.Logger, token string, appID int, appHash string) *Client {
	if appID == 0 {
		appID = 2040 // Default public Telegram Desktop API ID
	}
	if appHash == "" {
		appHash = "b18441a1ff607e10a989891a5462e627" // Default public Telegram Desktop API Hash
	}
	return &Client{
		log:         log,
		token:       token,
		appID:       appID,
		appHash:     appHash,
		channelHash: make(map[int64]int64),
	}
}

// Start launches MTProto connection in the background.
func (c *Client) Start(ctx context.Context) error {
	const op = "mtproto.Client.Start"

	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel

	readyCh := make(chan struct{})

	go func() {
		sessionPath := filepath.Join(os.TempDir(), "mgkct_bot_mtproto.session")

		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			client := telegram.NewClient(c.appID, c.appHash, telegram.Options{
				SessionStorage: &session.FileStorage{Path: sessionPath},
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

// SetInlineBotResultsWidgetRequest serializes messages.setInlineBotResults#bb12a419
// containing inputBotInlineMessageWidget#f597a3f0 (Telegram Layer 233).
type SetInlineBotResultsWidgetRequest struct {
	QueryID     int64
	ID          string
	Title       string
	Description string
	HTML        string
	CacheTime   int
	Private     bool
}

// Encode serializes messages.setInlineBotResults with inputBotInlineMessageWidget.
func (r *SetInlineBotResultsWidgetRequest) Encode(b *bin.Buffer) error {
	b.PutID(0xbb12a419) // messages.setInlineBotResults#bb12a419

	var flags bin.Fields
	if r.Private {
		flags.Set(1) // private
	}
	if err := flags.Encode(b); err != nil {
		return err
	}

	b.PutLong(r.QueryID)

	// results: Vector<InputBotInlineResult>
	b.PutVectorHeader(1)

	// inputBotInlineResult#88bf9319
	b.PutID(0x88bf9319)
	var resFlags bin.Fields
	resFlags.Set(1) // title
	resFlags.Set(2) // description
	if err := resFlags.Encode(b); err != nil {
		return err
	}
	b.PutString(r.ID)
	b.PutString("widget")
	b.PutString(r.Title)
	b.PutString(r.Description)

	// send_message: inputBotInlineMessageWidget#f597a3f0
	b.PutID(0xf597a3f0)
	var msgFlags bin.Fields
	if err := msgFlags.Encode(b); err != nil {
		return err
	}
	b.PutInt(0)         // width (0 = auto)
	b.PutInt(0)         // height (0 = auto)
	b.PutString(r.HTML) // html
	b.PutString("")     // message (empty string = no text caption below widget)

	b.PutInt(r.CacheTime)
	return nil
}

// SetInlineWidgetAnswer answers an inline query with a native interactive widget (inputBotInlineMessageWidget#f597a3f0).
func (c *Client) SetInlineWidgetAnswer(ctx context.Context, queryID int64, resultID, title, description, html string, cacheTime int, isPersonal bool) error {
	const op = "mtproto.Client.SetInlineWidgetAnswer"

	if !c.IsReady() {
		return fmt.Errorf("%s: MTProto client is not ready", op)
	}

	req := &SetInlineBotResultsWidgetRequest{
		QueryID:     queryID,
		ID:          resultID,
		Title:       title,
		Description: description,
		HTML:        html,
		CacheTime:   cacheTime,
		Private:     isPersonal,
	}

	var res tg.BoolBox
	if err := c.tgCl.Invoke(ctx, req, &res); err != nil {
		return fmt.Errorf("%s: invoke setInlineBotResults: %w", op, err)
	}

	c.log.Info("native inline widget answer set via MTProto", "operation", op, "query_id", queryID, "result_id", resultID)
	return nil
}
