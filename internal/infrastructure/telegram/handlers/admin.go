package handlers

import (
	"slices"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
)

// IsAdmin проверяет, является ли указанный chatID администратором бота.
func IsAdmin(adminIDs []int64, chatID int64) bool {
	return slices.Contains(adminIDs, chatID)
}

// AdminOnly возвращает middleware, которое блокирует выполнение хендлера для неадминистраторов,
// отвечая стандартным сообщением справки Help (как при неизвестной команде).
func AdminOnly(adminIDs []int64) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if !IsAdmin(adminIDs, c.Chat().ID) {
				return c.Send(messages.Help, keyboard.ReplyScheduleKeyboard)
			}
			return next(c)
		}
	}
}
