package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
)

// Notify обрабатывает команду /notify [on|off]
func Notify(log *slog.Logger, userRepo UserRepo) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		args := c.Args()
		if len(args) > 0 {
			arg := strings.ToLower(args[0])
			if arg == "on" || arg == "1" || arg == "true" {
				if err := userRepo.SetNotifyEvening(ctx, c.Chat().ID, true); err != nil {
					log.Error("failed setting notify evening on", "error", err)
					return c.Send("Ошибка сохранения настроек уведомлений.", keyboard.ReplyScheduleKeyboard)
				}
				return c.Send("🔔 Рассылка расписания на завтра *включена* (после окончания пар и вечером в 20:00).", tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
			}
			if arg == "off" || arg == "0" || arg == "false" {
				if err := userRepo.SetNotifyEvening(ctx, c.Chat().ID, false); err != nil {
					log.Error("failed setting notify evening off", "error", err)
					return c.Send("Ошибка сохранения настроек уведомлений.", keyboard.ReplyScheduleKeyboard)
				}
				return c.Send("🔕 Рассылка расписания на завтра *отключена*.", tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
			}
		}

		enabled, err := userRepo.GetNotifyEvening(ctx, c.Chat().ID)
		if err != nil {
			enabled = true
		}

		statusStr := "🔔 Включена (после окончания пар и в 20:00)"
		if !enabled {
			statusStr = "🔕 Отключена"
		}

		msg := fmt.Sprintf(
			"⚙️ *Настройки уведомлений*\n\nРассылка расписания на следующий день:\n*Статус:* %s\n\nВы можете переключить статус кнопкой ниже или командами:\n• `/notify on` — включить\n• `/notify off` — отключить",
			statusStr,
		)

		return c.Send(msg, tele.ModeMarkdown, keyboard.InlineNotifyKeyboard(enabled))
	}
}

func handleNotifyToggle(ctx context.Context, log *slog.Logger, c tele.Context, data string, userRepo UserRepo) error {
	raw := dataFromCallbackData(data)
	enable := raw == "on"

	if err := userRepo.SetNotifyEvening(ctx, c.Chat().ID, enable); err != nil {
		log.Error("failed toggling notify evening", "error", err)
		return err
	}

	statusStr := "🔔 Включена (после окончания пар и в 20:00)"
	if !enable {
		statusStr = "🔕 Отключена"
	}

	msg := fmt.Sprintf(
		"⚙️ *Настройки уведомлений*\n\nРассылка расписания на следующий день:\n*Статус:* %s\n\nВы можете переключить статус кнопкой ниже или командами:\n• `/notify on` — включить\n• `/notify off` — отключить",
		statusStr,
	)

	return safeEdit(c, msg, tele.ModeMarkdown, keyboard.InlineNotifyKeyboard(enable))
}
