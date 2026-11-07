package widget

import (
	"context"
	"log/slog"

	tele "gopkg.in/telebot.v4"
)

// SendWidgetOrFallback validates the widget HTML against Telegram Profile 1 size constraints
// and delivers the fallback message with the WebApp/navigation reply markup when native MTProto is unavailable.
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
	}

	return c.Send(fallbackText, sendOpts...)
}
