package handlers

import (
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
	tele "gopkg.in/telebot.v4"
)

func Calls() tele.HandlerFunc {
	return func(c tele.Context) error {
		return c.Send(messages.Calls, tele.ModeMarkdown)
	}
}
