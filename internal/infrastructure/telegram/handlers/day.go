package handlers

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
)

func Day(log *slog.Logger, schedSvc ScheduleService, userRepo UserRepo) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var (
			groupID int
			err     error
		)

		args := c.Args()
		if len(args) > 0 {
			groupID, err = strconv.Atoi(args[0])
			if err != nil || groupID <= 0 {
				return c.Send(messages.OnlyNumbers)
			}
		} else {
			groupID, err = userRepo.GroupByChatID(ctx, c.Chat().ID)
			if err != nil {
				return c.Send(formatter.FormatErrorMessage(err))
			}
		}

		group, err := schedSvc.GetGroupLatestSchedule(ctx, int32(groupID))
		if err != nil {
			log.Warn("failed getting group schedule", "group_id", groupID, "error", err)
			return c.Send(formatter.FormatErrorMessage(err))
		}

		msg := formatter.FormatScheduleDay(group)
		dayIdx := formatter.EffectiveDayIndex(group)

		return c.Send(
			msg,
			tele.ModeMarkdown,
			keyboard.ReplyScheduleKeyboard,
			keyboard.InlineDayKeyboard(int(group.ID), dayIdx, len(group.Days)),
		)
	}
}

func Week(log *slog.Logger, schedSvc ScheduleService, userRepo UserRepo) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var (
			groupID int
			err     error
		)

		args := c.Args()
		if len(args) > 0 {
			groupID, err = strconv.Atoi(args[0])
			if err != nil || groupID <= 0 {
				return c.Send(messages.OnlyNumbers)
			}
		} else {
			groupID, err = userRepo.GroupByChatID(ctx, c.Chat().ID)
			if err != nil {
				return c.Send(formatter.FormatErrorMessage(err))
			}
		}

		group, err := schedSvc.GetGroupLatestSchedule(ctx, int32(groupID))
		if err != nil {
			log.Warn("failed getting group schedule for week", "group_id", groupID, "error", err)
			return c.Send(formatter.FormatErrorMessage(err))
		}

		msg := formatter.FormatScheduleWeek(group)
		weeks, err := schedSvc.GetAvailableWeeks(ctx, group.Week)
		if err != nil {
			weeks = entities.WeekNavigation{}
		}

		return sendLongMessage(
			c,
			msg,
			tele.ModeMarkdown,
			keyboard.ReplyScheduleKeyboard,
			keyboard.InlineWeekKeyboard(int(group.ID), weeks),
		)
	}
}
