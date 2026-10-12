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

func CallbacksHandler(
	log *slog.Logger,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	analyticsSvc AnalyticsService,
	userRepo UserRepo,
) tele.HandlerFunc {
	return func(c tele.Context) error {
		callback := c.Callback()
		data := callback.Data

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var err error
		switch {
		// Уведомления
		case strings.Contains(data, keyboard.NotifyToggle):
			if userRepo != nil {
				err = handleNotifyToggle(ctx, log, c, data, userRepo)
			}

		// Кабинеты
		case strings.Contains(data, keyboard.CabinetPrevDay), strings.Contains(data, keyboard.CabinetNextDay):
			if analyticsSvc != nil {
				err = handleCabinetDayNav(ctx, log, c, data, analyticsSvc)
			}
		case strings.Contains(data, keyboard.CabinetCurrentWeek):
			if analyticsSvc != nil {
				err = handleCabinetWeek(ctx, log, c, data, analyticsSvc)
			}

		// Архив
		case strings.Contains(data, keyboard.ArchiveSelect):
			err = handleArchiveSelect(ctx, log, c, data, schedSvc, teacherSvc)

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
	teacherName, dayStr, found := strings.CutLast(raw, ":")
	if !found {
		return fmt.Errorf("invalid teacher day nav data: %s", raw)
	}

	dayIdx, err := strconv.Atoi(dayStr)
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

	member, _ := svc.GetFacultyMember(ctx, teacherName)
	msg = formatter.FormatTeacherProfile(member) + msg

	return safeEdit(c, msg, tele.ModeMarkdown, keyboard.InlineTeacherDayKeyboard(teacherName, dayIdx, len(teacher.Days)))
}

func handleTeacherCurrentWeek(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc TeacherService) error {
	teacherName := dataFromCallbackData(data)

	teacher, err := svc.GetTeacherLatestSchedule(ctx, teacherName)
	if err != nil {
		return err
	}

	member, _ := svc.GetFacultyMember(ctx, teacherName)
	msg := formatter.FormatTeacherProfile(member) + formatter.FormatTeacherScheduleWeek(teacher)
	weeks, err := svc.GetAvailableWeeks(ctx, teacherName, teacher.Week)
	if err != nil {
		return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}

	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineTeacherWeekKeyboard(teacherName, weeks))
}

func handleTeacherWeekNav(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc TeacherService) error {
	raw := dataFromCallbackData(data)
	teacherName, dateStr, found := strings.CutLast(raw, ":")
	if !found {
		return fmt.Errorf("invalid teacher week nav data: %s", raw)
	}

	date, err := time.Parse("02.01.2006", dateStr)
	if err != nil {
		return err
	}

	teacher, err := svc.GetTeacherScheduleByWeek(ctx, teacherName, date)
	if err != nil {
		return err
	}

	member, _ := svc.GetFacultyMember(ctx, teacherName)
	msg := formatter.FormatTeacherProfile(member) + formatter.FormatTeacherScheduleWeek(teacher)
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

func handleCabinetDayNav(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc AnalyticsService) error {
	raw := dataFromCallbackData(data)
	cabinet, dayStr, found := strings.CutLast(raw, ":")
	if !found {
		return fmt.Errorf("invalid cabinet day nav data: %s", raw)
	}

	dayIdx, err := strconv.Atoi(dayStr)
	if err != nil {
		return err
	}

	sched, _, err := svc.GetCabinetSchedule(ctx, cabinet, time.Time{})
	if err != nil {
		return err
	}
	if sched == nil {
		return fmt.Errorf("cabinet schedule not found for %s", cabinet)
	}
	if dayIdx < 0 || dayIdx >= len(sched.Days) {
		return fmt.Errorf("day index %d out of range (total %d)", dayIdx, len(sched.Days))
	}

	msg := formatter.FormatCabinetDay(sched, dayIdx)
	return safeEdit(c, msg, tele.ModeMarkdown, keyboard.InlineCabinetDayKeyboard(cabinet, dayIdx, len(sched.Days)))
}

func handleCabinetWeek(ctx context.Context, log *slog.Logger, c tele.Context, data string, svc AnalyticsService) error {
	cabinet := dataFromCallbackData(data)
	sched, _, err := svc.GetCabinetSchedule(ctx, cabinet, time.Time{})
	if err != nil {
		return err
	}
	if sched == nil {
		return fmt.Errorf("cabinet schedule not found for %s", cabinet)
	}

	msg := formatter.FormatCabinetWeek(sched)
	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
}

func handleArchiveSelect(ctx context.Context, log *slog.Logger, c tele.Context, data string, schedSvc ScheduleService, teacherSvc TeacherService) error {
	raw := dataFromCallbackData(data)
	// формат "g:101:02.01.2006" или "t:Иванов И.И.:02.01.2006"
	parts := strings.Split(raw, ":")
	if len(parts) < 3 {
		return fmt.Errorf("invalid archive select data: %s", raw)
	}

	targetType := parts[0]
	targetID := parts[1]
	dateStr := parts[2]

	date, err := time.Parse("02.01.2006", dateStr)
	if err != nil {
		return err
	}

	if targetType == "g" {
		groupID, err := strconv.Atoi(targetID)
		if err != nil {
			return err
		}
		group, err := schedSvc.GetArchiveSchedule(ctx, int32(groupID), date)
		if err != nil {
			return err
		}
		msg := formatter.FormatScheduleWeek(group)
		return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}

	teacher, err := teacherSvc.GetTeacherArchiveSchedule(ctx, targetID, date)
	if err != nil {
		return err
	}
	member, _ := teacherSvc.GetFacultyMember(ctx, targetID)
	msg := formatter.FormatTeacherProfile(member) + formatter.FormatTeacherScheduleWeek(teacher)
	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
}
