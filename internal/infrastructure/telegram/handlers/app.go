package handlers

import (
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
)

func App(webAppURL string) tele.HandlerFunc {
	return func(c tele.Context) error {
		if webAppURL == "" {
			return c.Send("Веб-приложение ещё не настроено (BOT_WEB_APP_URL не указан).", keyboard.ReplyScheduleKeyboard)
		}
		return c.Send("Нажмите кнопку ниже, чтобы открыть расписание в Telegram Mini App:", keyboard.InlineWebAppKeyboard(webAppURL))
	}
}
