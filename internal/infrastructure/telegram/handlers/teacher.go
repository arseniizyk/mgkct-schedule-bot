package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
)

func TeacherDay(log *slog.Logger, teacherSvc TeacherService, userRepo UserRepo) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var (
			teacherName string
			err         error
		)

		args := c.Args()
		if len(args) > 0 {
			query := strings.Join(args, " ")
			matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, query)
			if !ok {
				if len(candidates) > 0 {
					return c.Send(fmt.Sprintf(messages.TeacherAmbiguous, formatCandidates(candidates)))
				}
				return c.Send(messages.TeacherNotFound)
			}
			teacherName = matched
		} else {
			teacherName, err = userRepo.GetTeacher(ctx, c.Chat().ID)
			if err != nil {
				return c.Send(formatter.FormatErrorMessage(err))
			}
		}

		teacher, err := teacherSvc.GetTeacherLatestSchedule(ctx, teacherName)
		if err != nil {
			log.Warn("failed getting teacher schedule", "name", teacherName, "error", err)
			return c.Send(formatter.FormatErrorMessage(err))
		}

		member, _ := teacherSvc.GetFacultyMember(ctx, teacherName)
		msg := formatter.FormatTeacherProfile(member) + formatter.FormatTeacherScheduleDay(teacher)
		dayIdx := formatter.EffectiveTeacherDayIndex(teacher)

		return c.Send(
			msg,
			tele.ModeMarkdown,
			keyboard.ReplyScheduleKeyboard,
			keyboard.InlineTeacherDayKeyboard(teacher.Name, dayIdx, len(teacher.Days)),
		)
	}
}

func TeacherWeek(log *slog.Logger, teacherSvc TeacherService, userRepo UserRepo) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var (
			teacherName string
			err         error
		)

		args := c.Args()
		if len(args) > 0 {
			query := strings.Join(args, " ")
			matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, query)
			if !ok {
				if len(candidates) > 0 {
					return c.Send(fmt.Sprintf(messages.TeacherAmbiguous, formatCandidates(candidates)))
				}
				return c.Send(messages.TeacherNotFound)
			}
			teacherName = matched
		} else {
			teacherName, err = userRepo.GetTeacher(ctx, c.Chat().ID)
			if err != nil {
				return c.Send(formatter.FormatErrorMessage(err))
			}
		}

		teacher, err := teacherSvc.GetTeacherLatestSchedule(ctx, teacherName)
		if err != nil {
			log.Warn("failed getting teacher week schedule", "name", teacherName, "error", err)
			return c.Send(formatter.FormatErrorMessage(err))
		}

		member, _ := teacherSvc.GetFacultyMember(ctx, teacherName)
		msg := formatter.FormatTeacherProfile(member) + formatter.FormatTeacherScheduleWeek(teacher)
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
	}
}

func Teachers(log *slog.Logger, teacherSvc TeacherService, sm StateManager) tele.HandlerFunc {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) == 0 {
			if err := sm.Set(c.Chat().ID, state.WaitingTeacherSchedule); err != nil {
				log.Error("failed setting waiting teacher schedule state", "error", err)
				return c.Send(messages.Internal)
			}
			return c.Send(messages.WaitingTeacherSchedule)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		teacherQuery := strings.Join(args, " ")
		matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, teacherQuery)
		if !ok {
			if len(candidates) > 0 {
				return c.Send(fmt.Sprintf(messages.TeacherAmbiguous, formatCandidates(candidates)))
			}
			return c.Send(messages.TeacherNotFound)
		}

		teacher, err := teacherSvc.GetTeacherLatestSchedule(ctx, matched)
		if err != nil {
			log.Warn("failed getting teacher week schedule", "name", matched, "error", err)
			return c.Send(formatter.FormatErrorMessage(err))
		}

		member, _ := teacherSvc.GetFacultyMember(ctx, matched)
		msg := formatter.FormatTeacherProfile(member) + formatter.FormatTeacherScheduleWeek(teacher)
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
	}
}

func SetTeacher(log *slog.Logger, teacherSvc TeacherService, userRepo UserRepo, sm StateManager) tele.HandlerFunc {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) == 0 {
			if err := sm.Set(c.Chat().ID, state.WaitingTeacher); err != nil {
				log.Error("failed setting waiting teacher state", "error", err)
				return c.Send(messages.Internal)
			}
			return c.Send(messages.WaitingTeacher)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		teacherQuery := strings.Join(args, " ")
		matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, teacherQuery)
		if !ok {
			if len(candidates) > 0 {
				return c.Send(fmt.Sprintf(messages.TeacherAmbiguous, formatCandidates(candidates)))
			}
			return c.Send(messages.TeacherNotFound)
		}

		if err := userRepo.SetTeacher(ctx, c.Chat().ID, matched); err != nil {
			log.Error("failed saving user teacher", "name", matched, "error", err)
			return c.Send(messages.Internal)
		}

		return c.Send(messages.TeacherSaved, keyboard.ReplyScheduleKeyboard)
	}
}

func formatCandidates(candidates []string) string {
	var sb strings.Builder
	for _, c := range candidates {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString("- " + c)
	}
	return sb.String()
}
