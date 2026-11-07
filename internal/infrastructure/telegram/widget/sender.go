package widget

import (
	"context"
	"encoding/json"
	"log/slog"

	tele "gopkg.in/telebot.v4"
)

// RawExecutor represents any bot client capable of executing raw Bot API calls.
type RawExecutor interface {
	Raw(method string, payload any) ([]byte, error)
}

// RawResponse represents standard Telegram Bot API response structure.
type RawResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
	Description string          `json:"description,omitempty"`
}

// SendWidgetOrFallback attempts to send an interactive Mini-HTML widget via Bot API (sendWidget).
// If the Bot API server does not support sendWidget (e.g. returns 404 before universal Bot API 10.4 rollout)
// or returns an error, it seamlessly falls back to standard message delivery via tele.Context.
func SendWidgetOrFallback(
	_ context.Context,
	log *slog.Logger,
	c tele.Context,
	widgetHTML string,
	fallbackText string,
	replyMarkup *tele.ReplyMarkup,
	opts ...any,
) error {
	const op = "widget.SendWidgetOrFallback"

	sendOpts := make([]any, 0, len(opts)+2)
	sendOpts = append(sendOpts, tele.ModeMarkdown)
	if replyMarkup != nil {
		sendOpts = append(sendOpts, replyMarkup)
	}
	sendOpts = append(sendOpts, opts...)

	// Validate widget size constraints (max 32 KB uncompressed, max 4 KB zlib)
	if _, _, err := ValidateSize(widgetHTML); err != nil {
		log.Warn("widget payload exceeds limits, falling back to message", "operation", op, "error", err)
		return c.Send(fallbackText, sendOpts...)
	}

	// Attempt native sendWidget call if bot supports Raw calls
	if c != nil && c.Bot() != nil && c.Recipient() != nil {
		if rawer, ok := c.Bot().(RawExecutor); ok {
			payload := map[string]any{
				"chat_id": c.Recipient().Recipient(),
				"html":    widgetHTML,
				"caption": fallbackText,
			}
			if replyMarkup != nil {
				payload["reply_markup"] = replyMarkup
			}

			rawResp, err := rawer.Raw("sendWidget", payload)
			if err == nil {
				var resp RawResponse
				if jsonErr := json.Unmarshal(rawResp, &resp); jsonErr == nil && resp.OK {
					log.Info("widget sent successfully via sendWidget", "operation", op, "chat_id", c.Recipient().Recipient())
					return nil
				}
			}
			log.Debug("sendWidget call not available or failed, using fallback message", "operation", op, "error", err)
		}
	}

	return c.Send(fallbackText, sendOpts...)
}
