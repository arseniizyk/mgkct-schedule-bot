package handlers

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
)

func Day(log *slog.Logger, schedSvc ScheduleService, teacherSvc TeacherService, userRepo UserRepo) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		args := c.Args()
		if len(args) > 0 {
			// Если аргумент число — расписание группы
			groupID, err := strconv.Atoi(args[0])
			if err == nil && groupID > 0 {
				group, err := schedSvc.GetGroupLatestSchedule(ctx, int32(groupID))
				if err != nil {
					log.Warn("failed getting group schedule", "group_id", groupID, "error", err)
					return c.Send(formatter.FormatErrorMessage(err))
				}

				msg := formatter.FormatScheduleDay(group)
				dayIdx := formatter.EffectiveDayIndex(group)
				weeks, _ := schedSvc.GetAvailableWeeks(ctx, group.Week)
				if weeks.Current.IsZero() {
					weeks.Current = group.Week
				}

				return c.Send(
					msg,
					tele.ModeMarkdown,
					keyboard.ReplyScheduleKeyboard,
					keyboard.InlineDayKeyboard(int(group.ID), dayIdx, len(group.Days), 0, weeks),
				)
			}

			// Иначе пробуем найти преподавателя по фамилии
			query := strings.Join(args, " ")
			matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, query)
			if !ok {
				return sendTeacherNotFoundOrSuggestions(ctx, c, teacherSvc, query, candidates)
			}

			teacher, err := teacherSvc.GetTeacherLatestSchedule(ctx, matched)
			if err != nil {
				log.Warn("failed getting teacher schedule", "name", matched, "error", err)
				return c.Send(formatter.FormatErrorMessage(err))
			}

			msg := formatter.FormatTeacherScheduleDay(teacher)
			dayIdx := formatter.EffectiveTeacherDayIndex(teacher)
			weeks, _ := teacherSvc.GetAvailableWeeks(ctx, teacher.Name, teacher.Week)
			if weeks.Current.IsZero() {
				weeks.Current = teacher.Week
			}

			return c.Send(
				msg,
				tele.ModeMarkdown,
				keyboard.ReplyScheduleKeyboard,
				keyboard.InlineTeacherDayKeyboard(teacher.Name, dayIdx, len(teacher.Days), weeks),
			)
		}

		// Без аргументов: если у пользователя установлен преподаватель, отдаем его расписание
		teacherName, err := userRepo.GetTeacher(ctx, c.Chat().ID)
		if err == nil && teacherName != "" {
			teacher, err := teacherSvc.GetTeacherLatestSchedule(ctx, teacherName)
			if err != nil {
				log.Warn("failed getting teacher schedule", "name", teacherName, "error", err)
				return c.Send(formatter.FormatErrorMessage(err))
			}

			msg := formatter.FormatTeacherScheduleDay(teacher)
			dayIdx := formatter.EffectiveTeacherDayIndex(teacher)
			weeks, _ := teacherSvc.GetAvailableWeeks(ctx, teacher.Name, teacher.Week)
			if weeks.Current.IsZero() {
				weeks.Current = teacher.Week
			}

			return c.Send(
				msg,
				tele.ModeMarkdown,
				keyboard.ReplyScheduleKeyboard,
				keyboard.InlineTeacherDayKeyboard(teacher.Name, dayIdx, len(teacher.Days), weeks),
			)
		}

		// Иначе запрашиваем группу студента
		groupID, subgroup, err := userRepo.GroupAndSubgroupByChatID(ctx, c.Chat().ID)
		if err != nil {
			return c.Send(formatter.FormatErrorMessage(err))
		}

		group, err := schedSvc.GetGroupLatestSchedule(ctx, int32(groupID))
		if err != nil {
			log.Warn("failed getting group schedule", "group_id", groupID, "error", err)
			return c.Send(formatter.FormatErrorMessage(err))
		}

		filtered := formatter.FilterGroupForSubgroup(group, subgroup)
		msg := formatter.FormatScheduleDay(filtered)
		dayIdx := formatter.EffectiveDayIndex(filtered)
		weeks, _ := schedSvc.GetAvailableWeeks(ctx, group.Week)
		if weeks.Current.IsZero() {
			weeks.Current = group.Week
		}

		return c.Send(
			msg,
			tele.ModeMarkdown,
			keyboard.ReplyScheduleKeyboard,
			keyboard.InlineDayKeyboard(int(group.ID), dayIdx, len(group.Days), subgroup, weeks),
		)
	}
}

func Week(log *slog.Logger, schedSvc ScheduleService, teacherSvc TeacherService, userRepo UserRepo) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		args := c.Args()
		if len(args) > 0 {
			// Если аргумент число — расписание группы на неделю
			groupID, err := strconv.Atoi(args[0])
			if err == nil && groupID > 0 {
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
					keyboard.InlineWeekKeyboard(int(group.ID), weeks, 0),
				)
			}

			// Иначе пробуем найти преподавателя по фамилии
			query := strings.Join(args, " ")
			matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, query)
			if !ok {
				return sendTeacherNotFoundOrSuggestions(ctx, c, teacherSvc, query, candidates)
			}

			teacher, err := teacherSvc.GetTeacherLatestSchedule(ctx, matched)
			if err != nil {
				log.Warn("failed getting teacher week schedule", "name", matched, "error", err)
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
		}

		// Без аргументов: если у пользователя установлен преподаватель, отдаем его расписание
		teacherName, err := userRepo.GetTeacher(ctx, c.Chat().ID)
		if err == nil && teacherName != "" {
			teacher, err := teacherSvc.GetTeacherLatestSchedule(ctx, teacherName)
			if err != nil {
				log.Warn("failed getting teacher week schedule", "name", teacherName, "error", err)
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
		}

		// Иначе запрашиваем группу студента
		groupID, subgroup, err := userRepo.GroupAndSubgroupByChatID(ctx, c.Chat().ID)
		if err != nil {
			return c.Send(formatter.FormatErrorMessage(err))
		}

		group, err := schedSvc.GetGroupLatestSchedule(ctx, int32(groupID))
		if err != nil {
			log.Warn("failed getting group schedule for week", "group_id", groupID, "error", err)
			return c.Send(formatter.FormatErrorMessage(err))
		}

		msg := formatter.FormatScheduleWeekForSubgroup(group, subgroup)
		weeks, err := schedSvc.GetAvailableWeeks(ctx, group.Week)
		if err != nil {
			weeks = entities.WeekNavigation{}
		}

		return sendLongMessage(
			c,
			msg,
			tele.ModeMarkdown,
			keyboard.ReplyScheduleKeyboard,
			keyboard.InlineWeekKeyboard(int(group.ID), weeks, subgroup),
		)
	}
}
