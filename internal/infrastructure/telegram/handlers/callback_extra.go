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
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
)

func dataFromCallbackData(data string) string {
	parts := strings.Split(data, "|")
	if len(parts) > 1 {
		return parts[1]
	}
	return ""
}

func handleRoleSelect(ctx context.Context, log *slog.Logger, c tele.Context, data string, sm StateManager) error {
	raw := dataFromCallbackData(data)
	chatID := callbackChatID(c)

	_ = c.Edit(&tele.ReplyMarkup{})

	switch raw {
	case keyboard.RoleStudent:
		if err := sm.Set(chatID, state.WaitingGroup); err != nil {
			log.Error("failed setting waiting group state from role select", "error", err)
			return c.Send(messages.Internal)
		}
		return c.Send("🎓 Вы выбрали роль: *Студент*\n\n"+messages.WaitingGroup, tele.ModeMarkdown)

	case keyboard.RoleTeacher:
		if err := sm.Set(chatID, state.WaitingTeacher); err != nil {
			log.Error("failed setting waiting teacher state from role select", "error", err)
			return c.Send(messages.Internal)
		}
		return c.Send("👨‍🏫 Вы выбрали роль: *Преподаватель*\n\n"+messages.WaitingTeacher, tele.ModeMarkdown)

	default:
		return fmt.Errorf("unknown role select data: %s", raw)
	}
}

func handleSubgroupSelect(ctx context.Context, log *slog.Logger, c tele.Context, data string, userRepo UserRepo, sm StateManager) error {
	raw := dataFromCallbackData(data)
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return fmt.Errorf("invalid subgroup select data: %s", raw)
	}

	groupID, err := strconv.Atoi(parts[0])
	if err != nil {
		return err
	}

	subgroup, err := strconv.Atoi(parts[1])
	if err != nil {
		return err
	}

	chatID := callbackChatID(c)
	if err := userRepo.SetUserGroup(ctx, chatID, groupID, subgroup); err != nil {
		log.Error("failed saving user group and subgroup", "group_id", groupID, "subgroup", subgroup, "error", err)
		return err
	}

	if sm != nil {
		_ = sm.Clear(chatID)
	}

	var text string
	if subgroup > 0 {
		text = fmt.Sprintf(messages.GroupWithSubgroupSaved, groupID, subgroup)
	} else {
		text = messages.GroupSaved
	}

	_ = c.Edit(&tele.ReplyMarkup{})
	return c.Send(text, keyboard.ReplyScheduleKeyboard)
}

func handleTeacherSetSelect(ctx context.Context, log *slog.Logger, c tele.Context, data string, userRepo UserRepo, sm StateManager) error {
	raw := dataFromCallbackData(data)
	teacherName := raw
	if teacherName == "" {
		return fmt.Errorf("invalid teacher set select data: %s", data)
	}

	chatID := callbackChatID(c)
	if err := userRepo.SetTeacher(ctx, chatID, teacherName); err != nil {
		log.Error("failed saving user teacher from callback", "teacher", teacherName, "error", err)
		return err
	}

	if sm != nil {
		_ = sm.Clear(chatID)
	}

	_ = c.Edit(&tele.ReplyMarkup{})
	return c.Send(messages.TeacherSaved, keyboard.ReplyScheduleKeyboard)
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
	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineCabinetWeekKeyboard(cabinet))
}

func handleArchiveSelect(ctx context.Context, log *slog.Logger, c tele.Context, data string, schedSvc ScheduleService, teacherSvc TeacherService, userRepo UserRepo) error {
	raw := dataFromCallbackData(data)
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

	kbd := keyboard.InlineArchiveReturnKeyboard(targetType, targetID)

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
		return editLongMessage(c, msg, tele.ModeMarkdown, kbd)
	}

	teacher, err := teacherSvc.GetTeacherArchiveSchedule(ctx, targetID, date)
	if err != nil {
		return err
	}
	var profile string
	if shouldShowTeacherProfile(ctx, userRepo, callbackChatID(c), targetID) {
		member, _ := teacherSvc.GetFacultyMember(ctx, targetID)
		profile = formatter.FormatTeacherProfile(member)
	}
	msg := profile + formatter.FormatTeacherScheduleWeek(teacher)
	return editLongMessage(c, msg, tele.ModeMarkdown, kbd)
}

func handleArchiveBack(ctx context.Context, log *slog.Logger, c tele.Context, data string, schedSvc ScheduleService, teacherSvc TeacherService) error {
	raw := dataFromCallbackData(data)
	parts := strings.Split(raw, ":")
	if len(parts) < 2 {
		return fmt.Errorf("invalid archive back data: %s", raw)
	}

	targetType := parts[0]
	targetID := parts[1]

	if targetType == "g" {
		groupID, err := strconv.Atoi(targetID)
		if err != nil {
			return err
		}
		weeks, err := schedSvc.GetAllWeeks(ctx)
		if err != nil || len(weeks) == 0 {
			return safeEdit(c, fmt.Sprintf("Архив расписаний для группы %d пока пуст.", groupID))
		}
		msg := fmt.Sprintf("📦 *Архив расписания группы %d*\nВыберите неделю для просмотра:", groupID)
		kbd := keyboard.InlineArchiveWeeksKeyboard("g", targetID, weeks)
		return safeEdit(c, msg, tele.ModeMarkdown, kbd)
	}

	weeks, err := teacherSvc.GetAllWeeks(ctx, targetID)
	if err != nil || len(weeks) == 0 {
		return safeEdit(c, fmt.Sprintf("📦 Архив расписаний для преподавателя *%s* пока пуст.", targetID), tele.ModeMarkdown)
	}
	msg := fmt.Sprintf("📦 *Архив расписания: %s*\nВыберите неделю для просмотра:", targetID)
	kbd := keyboard.InlineArchiveWeeksKeyboard("t", targetID, weeks)
	return safeEdit(c, msg, tele.ModeMarkdown, kbd)
}
