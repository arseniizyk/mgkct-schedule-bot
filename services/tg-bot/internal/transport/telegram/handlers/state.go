package handlers

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/state"
	"gopkg.in/telebot.v4"
)

func StatesHandler(log *slog.Logger, groupSetter UserGroupSetter, stateGetter StateGetter, stateClearer StateClearer) telebot.HandlerFunc {
	return func(c telebot.Context) error {
		chatID := c.Chat().ID

		log := log.With(
			"operation", "transport.telegram.handlers.State",
			"chat_id", chatID,
		)

		userState, ok := stateGetter.Get(chatID)
		if !ok { // if user write something wrong, not a command or callback
			return c.Send(messages.Help)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		switch userState {
		case state.WaitingGroup:
			groupID, err := strconv.Atoi(c.Text())
			if err != nil {
				log.Error("failed to parse group input to int", "text", c.Text(), "error", err)
				return c.Send(messages.OnlyNumbers)
			}

			if err := groupSetter.SetUserGroup(ctx, chatID, groupID); err != nil {
				log.Error("failed to set user's group", "group_id", groupID, "error", err)
				return c.Send(messages.InternalTryWith)
			}

			if err := stateClearer.Clear(chatID); err != nil {
				slog.Error("failed to clear user's state", "error", err)
			}

			return c.Send(messages.GroupSaved, keyboard.ReplyScheduleKeyboard)

		default:
			log.Warn("unxpected user state", "state", userState)
			if err := stateClearer.Clear(chatID); err != nil {
				log.Error("failed to clear unexpected state", "state", userState, "error", err)
				return nil
			}
			return c.Send(messages.Cancelled)
		}
	}
}
