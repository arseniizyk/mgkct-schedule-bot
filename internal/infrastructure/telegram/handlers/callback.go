package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
)

func CallbacksHandler(log *slog.Logger, schedSvc ScheduleService, teacherSvc TeacherService) tele.HandlerFunc {
	return func(c tele.Context) error {
		callback := c.Callback()
		data := callback.Data

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var err error
		switch {
		// Преподаватели (ОБЯЗАТЕЛЬНО перед группами, чтобы t_currweek/t_prevday не попадали в обработчик групп!)
		case strings.Contains(data, keyboard.TeacherPrevDay), strings.Contains(data, keyboard.TeacherNextDay):
			err = handleTeacherDayNav(ctx, log, c, data, teacherSvc)
		case strings.Contains(data, keyboard.TeacherCurrentWeek):
			err = handleTeacherCurrentWeek(ctx, log, c, data, teacherSvc)
		case strings.Contains(data, keyboard.TeacherPrevWeek), strings.Contains(data, keyboard.TeacherNextWeek):
			err = handleTeacherWeekNav(ctx, log, c, data, teacherSvc)

		// Группы
		case strings.Contains(data, keyboard.PrevDay), strings.Contains(data, keyboard.NextDay),
			strings.Contains(data, "prevday"), strings.Contains(data, "nextday"):
			err = handleDayNav(ctx, log, c, data, schedSvc)
		case strings.Contains(data, keyboard.CurrentWeek), strings.Contains(data, "currentweek"):
			err = handleCurrentWeek(ctx, log, c, data, schedSvc)
		case strings.Contains(data, keyboard.PrevWeek), strings.Contains(data, keyboard.NextWeek),
			strings.Contains(data, "prevweek"), strings.Contains(data, "nextweek"):
			err = handleWeekNav(ctx, log, c, data, schedSvc)

		default:
			log.Warn("unexpected callback", "data", data)
			return c.Respond(&tele.CallbackResponse{Text: messages.Internal, ShowAlert: true})
		}

		if err != nil {
			if strings.Contains(err.Error(), "message is not modified") {
				return c.Respond()
			}
			log.Error("callback execution error", "error", err, "data", data)
			return c.Respond(&tele.CallbackResponse{Text: messages.Internal, ShowAlert: true})
		}

		return c.Respond()
	}
}

func handleDayNav(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc ScheduleService) error {
	raw := dataFromCallbackData(data)
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return fmt.Errorf("invalid day nav data: %s", raw)
	}

	groupID, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}
	dayIdx, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}

	group, err := svc.GetGroupLatestSchedule(ctx, int32(groupID))
	if err != nil {
		return err
	}

	msg, err := formatter.FormatScheduleDayAt(group, dayIdx)
	if err != nil {
		return err
	}

	return safeEdit(c, msg, tele.ModeMarkdown, keyboard.InlineDayKeyboard(groupID, dayIdx, len(group.Days)))
}

func handleCurrentWeek(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc ScheduleService) error {
	raw := dataFromCallbackData(data)
	groupID, err := strconv.Atoi(raw)
	if err != nil {
		return err
	}

	group, err := svc.GetGroupLatestSchedule(ctx, int32(groupID))
	if err != nil {
		return err
	}

	msg := formatter.FormatScheduleWeek(group)
	weeks, err := svc.GetAvailableWeeks(ctx, group.Week)
	if err != nil {
		return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}

	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineWeekKeyboard(groupID, weeks))
}

func handleWeekNav(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc ScheduleService) error {
	raw := dataFromCallbackData(data)
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return fmt.Errorf("invalid week nav data: %s", raw)
	}

	groupID, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}

	date, err := time.Parse("02.01.2006", parts[1])
	if err != nil {
		return err
	}

	group, err := svc.GetGroupScheduleByWeek(ctx, int32(groupID), date)
	if err != nil {
		return err
	}

	msg := formatter.FormatScheduleWeek(group)
	weeks, err := svc.GetAvailableWeeks(ctx, date)
	if err != nil {
		return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}

	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineWeekKeyboard(groupID, weeks))
}

func handleTeacherDayNav(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc TeacherService) error {
	raw := dataFromCallbackData(data)
	idx := strings.LastIndex(raw, ":")
	if idx < 0 {
		return fmt.Errorf("invalid teacher day nav data: %s", raw)
	}

	teacherName := raw[:idx]
	dayIdx, err := strconv.Atoi(raw[idx+1:])
	if err != nil {
		return err
	}

	teacher, err := svc.GetTeacherLatestSchedule(ctx, teacherName)
	if err != nil {
		return err
	}

	msg, err := formatter.FormatTeacherScheduleDayAt(teacher, dayIdx)
	if err != nil {
		return err
	}

	return safeEdit(c, msg, tele.ModeMarkdown, keyboard.InlineTeacherDayKeyboard(teacherName, dayIdx, len(teacher.Days)))
}

func handleTeacherCurrentWeek(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc TeacherService) error {
	teacherName := dataFromCallbackData(data)

	teacher, err := svc.GetTeacherLatestSchedule(ctx, teacherName)
	if err != nil {
		return err
	}

	msg := formatter.FormatTeacherScheduleWeek(teacher)
	weeks, err := svc.GetAvailableWeeks(ctx, teacherName, teacher.Week)
	if err != nil {
		return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}

	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineTeacherWeekKeyboard(teacherName, weeks))
}

func handleTeacherWeekNav(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc TeacherService) error {
	raw := dataFromCallbackData(data)
	idx := strings.LastIndex(raw, ":")
	if idx < 0 {
		return fmt.Errorf("invalid teacher week nav data: %s", raw)
	}

	teacherName := raw[:idx]
	date, err := time.Parse("02.01.2006", raw[idx+1:])
	if err != nil {
		return err
	}

	teacher, err := svc.GetTeacherScheduleByWeek(ctx, teacherName, date)
	if err != nil {
		return err
	}

	msg := formatter.FormatTeacherScheduleWeek(teacher)
	weeks, err := svc.GetAvailableWeeks(ctx, teacherName, date)
	if err != nil {
		return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}

	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineTeacherWeekKeyboard(teacherName, weeks))
}

func dataFromCallbackData(data string) string {
	parts := strings.Split(data, "|")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}
