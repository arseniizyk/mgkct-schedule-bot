package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
)

func StatesHandler(log *slog.Logger, userRepo UserRepo, sm StateManager, teacherSvc TeacherService) tele.HandlerFunc {
	return func(c tele.Context) error {
		chatID := c.Chat().ID

		userState, ok := sm.Get(chatID)
		if !ok {
			return c.Send(messages.Help)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		switch userState {
		case state.WaitingGroup:
			groupID, err := strconv.Atoi(c.Text())
			if err != nil || groupID <= 0 {
				return c.Send(messages.OnlyNumbers)
			}

			if err := userRepo.SetUserGroup(ctx, chatID, groupID); err != nil {
				log.Error("failed saving group from state", "group_id", groupID, "error", err)
				return c.Send(messages.InternalTryWith)
			}

			_ = sm.Clear(chatID)
			return c.Send(messages.GroupSaved, keyboard.ReplyScheduleKeyboard)

		case state.WaitingTeacher:
			teacherQuery := c.Text()
			matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, teacherQuery)
			if !ok {
				if len(candidates) > 0 {
					return c.Send(fmt.Sprintf(messages.TeacherAmbiguous, formatCandidates(candidates)))
				}
				return c.Send(messages.TeacherNotFound)
			}

			if err := userRepo.SetTeacher(ctx, chatID, matched); err != nil {
				log.Error("failed saving teacher from state", "name", matched, "error", err)
				return c.Send(messages.Internal)
			}

			_ = sm.Clear(chatID)
			return c.Send(messages.TeacherSaved, keyboard.ReplyScheduleKeyboard)

		case state.WaitingTeacherSchedule:
			teacherQuery := c.Text()
			matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, teacherQuery)
			if !ok {
				if len(candidates) > 0 {
					return c.Send(fmt.Sprintf(messages.TeacherAmbiguous, formatCandidates(candidates)))
				}
				return c.Send(messages.TeacherNotFound)
			}

			_ = sm.Clear(chatID)

			teacher, err := teacherSvc.GetTeacherLatestSchedule(ctx, matched)
			if err != nil {
				log.Warn("failed getting teacher week schedule from state", "name", matched, "error", err)
				return c.Send(formatter.FormatErrorMessage(err))
			}

			msg := formatter.FormatTeacherScheduleWeek(teacher)
			weeks, err := teacherSvc.GetAvailableWeeks(ctx, teacher.Name, teacher.Week)
			if err != nil {
				weeks = entities.WeekNavigation{}
			}

			return sendLongMessage(
				c,
				msg,
				tele.ModeMarkdown,
				keyboard.ReplyScheduleKeyboard,
				keyboard.InlineTeacherWeekKeyboard(teacher.Name, weeks),
			)

		default:
			_ = sm.Clear(chatID)
			return c.Send(messages.Cancelled)
		}
	}
}
