package handlers

import (
	"log/slog"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
	tele "gopkg.in/telebot.v4"
)

func Cancel(log *slog.Logger, stateClearer StateClearer) tele.HandlerFunc {
	return func(c tele.Context) error {
		log := log.With(
			"operation", "transport.telegram.handlers.cancel.CancelHandler",
			"chat_id", c.Chat().ID,
		)

		if err := stateClearer.Clear(c.Chat().ID); err != nil {
			log.Error("failed to clear state", "error", err)
		}

		return c.Send(messages.Cancelled)
	}
}
