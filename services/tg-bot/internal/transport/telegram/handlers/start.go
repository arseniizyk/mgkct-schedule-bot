package handlers

import (
	"context"
	"log/slog"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
	tele "gopkg.in/telebot.v4"
)

func Start(log *slog.Logger, userSaver UserSaver) tele.HandlerFunc {
	return func(c tele.Context) error {
		user := entities.NewUser(c.Chat().ID, c.Sender().Username)
		log := log.With(
			"operation", "transport.telegram.handlers.start.StartHandler",
			"chat_id", user.ChatID,
			"username", user.Username,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := userSaver.SaveUser(ctx, user); err != nil {
			log.Error("can't save user from /start", "error", err)
		}

		return c.Send(messages.Start, tele.ModeMarkdown, tele.NoPreview)
	}
}
