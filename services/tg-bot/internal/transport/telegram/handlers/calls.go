package handlers

import (
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
)

func Calls() tele.HandlerFunc {
	return func(c tele.Context) error {
		return c.Send(messages.Calls, tele.ModeMarkdown)
	}
}
