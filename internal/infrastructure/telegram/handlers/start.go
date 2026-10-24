package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
)

func Start(log *slog.Logger, userRepo UserRepo) tele.HandlerFunc {
	return func(c tele.Context) error {
		user := entities.NewUser(c.Chat().ID, c.Sender().Username)
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := userRepo.SaveUser(ctx, user); err != nil {
			log.Warn("can't save user from /start", "error", err)
		}

		return c.Send(messages.Start, tele.ModeMarkdown, tele.NoPreview, keyboard.ReplyScheduleKeyboard)
	}
}

func Calls() tele.HandlerFunc {
	return func(c tele.Context) error {
		return c.Send(messages.Calls, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}
}

func Cancel(log *slog.Logger, sm StateManager) tele.HandlerFunc {
	return func(c tele.Context) error {
		if err := sm.Clear(c.Chat().ID); err != nil {
			log.Warn("failed to clear state on cancel", "error", err)
		}
		return c.Send(messages.Cancelled, keyboard.ReplyScheduleKeyboard)
	}
}

func SetGroup(log *slog.Logger, userRepo UserRepo, sm StateManager) tele.HandlerFunc {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) == 0 {
			if err := sm.Set(c.Chat().ID, state.WaitingGroup); err != nil {
				log.Error("failed setting state", "error", err)
				return c.Send(messages.Internal)
			}
			return c.Send(messages.WaitingGroup)
		}

		groupID, err := strconv.Atoi(args[0])
		if err != nil || groupID <= 0 {
			return c.Send(messages.OnlyNumbers)
		}

		if len(args) == 1 {
			return c.Send(fmt.Sprintf(messages.SelectSubgroup, groupID), keyboard.InlineSubgroupKeyboard(groupID))
		}

		subgroup, err := strconv.Atoi(args[1])
		if err != nil || subgroup < 0 || subgroup > 2 {
			return c.Send(fmt.Sprintf(messages.SelectSubgroup, groupID), keyboard.InlineSubgroupKeyboard(groupID))
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := userRepo.SetUserGroup(ctx, c.Chat().ID, groupID, subgroup); err != nil {
			log.Error("failed saving user group", "error", err)
			return c.Send(messages.Internal)
		}

		if subgroup > 0 {
			return c.Send(fmt.Sprintf(messages.GroupWithSubgroupSaved, groupID, subgroup), keyboard.ReplyScheduleKeyboard)
		}
		return c.Send(messages.GroupSaved, keyboard.ReplyScheduleKeyboard)
	}
}
