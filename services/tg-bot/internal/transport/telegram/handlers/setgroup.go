package handlers

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/state"
	tele "gopkg.in/telebot.v4"
)

func SetGroup(log *slog.Logger, groupSetter UserGroupSetter, stateSetter StateSetter) tele.HandlerFunc {
	return func(c tele.Context) error {
		log := log.With(
			"operation", "transport.telegram.handlers.SetGroup",
			"chat_id", c.Chat().ID,
		)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// if command used like /setgroup 99
		if len(c.Args()) > 0 {
			groupID, err := strconv.Atoi(c.Args()[0])
			if err != nil {
				log.Error("failed to parse user setgroup input", "args", c.Args(), "error", err)
				return c.Send(messages.OnlyNumbers)
			}

			if err := groupSetter.SetUserGroup(ctx, c.Chat().ID, groupID); err != nil {
				log.Error("failed to set group", "group_id", groupID, "error", err)
				return c.Send(messages.InternalTryWith)
			}

			return c.Send(messages.GroupSaved, keyboard.ReplyScheduleKeyboard)
		}

		if err := stateSetter.Set(c.Chat().ID, state.WaitingGroup); err != nil {
			log.Error("failed to set state", "state", state.WaitingGroup, "error", err)
			return c.Send(messages.InternalTryWith)
		}

		return c.Send(messages.WaitingGroup)
	}
}
