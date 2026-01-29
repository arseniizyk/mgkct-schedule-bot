package logger

import (
	"log/slog"

	tele "gopkg.in/telebot.v4"
)

func LogMessages(log *slog.Logger) func(next tele.HandlerFunc) tele.HandlerFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if c.Callback() != nil {
				log.Info("incoming callback",
					"chat_id", c.Chat().ID,
					"username", c.Sender().Username,
					"data", c.Callback().Data,
				)
			} else {
				log.Info("incoming message",
					"chat_id", c.Chat().ID,
					"username", c.Sender().Username,
					"text", c.Text())
			}
			return next(c)
		}
	}
}
