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

// Archive обрабатывает команду /archive [группа|преподаватель] [дата]
func Archive(
	log *slog.Logger,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	userRepo UserRepo,
	sm StateManager,
) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		args := c.Args()

		// 1. Без аргументов: смотрим сохраненный профиль пользователя
		if len(args) == 0 {
			return handleArchiveNoArgs(ctx, c, log, schedSvc, teacherSvc, userRepo, sm)
		}

		// 2. С аргументами: проверяем наличие даты в конце аргументов
		var targetDate time.Time
		var hasDate bool
		targetArgs := args

		if len(args) > 0 {
			if d, ok := parseArchiveDate(args[len(args)-1]); ok {
				targetDate = d
				hasDate = true
				targetArgs = args[:len(args)-1]
			}
		}

		// Если передан только один аргумент и это дата:
		if hasDate && len(targetArgs) == 0 {
			groupID, subgroup, teacher, _ := userRepo.GetUserProfile(ctx, c.Chat().ID)
			if groupID > 0 {
				return sendGroupArchiveSchedule(ctx, c, schedSvc, int32(groupID), subgroup, targetDate)
			}
			if teacher != "" {
				return sendTeacherArchiveSchedule(ctx, c, teacherSvc, userRepo, teacher, targetDate)
			}
			return c.Send("Укажите группу или преподавателя для поиска в архиве: `/archive <группа> <дата>`", tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
		}

		targetStr := strings.Join(targetArgs, " ")
		if !hasDate {
			return sendArchiveWeeksForTarget(ctx, c, schedSvc, teacherSvc, targetStr)
		}

		if gID, err := strconv.Atoi(targetStr); err == nil && gID > 0 {
			subgroup := 0
			if userRepo != nil {
				if userGrp, userSg, err := userRepo.GroupAndSubgroupByChatID(ctx, c.Chat().ID); err == nil && userGrp == gID {
					subgroup = userSg
				}
			}
			return sendGroupArchiveSchedule(ctx, c, schedSvc, int32(gID), subgroup, targetDate)
		}

		validatedName, candidates, ok := teacherSvc.ValidateTeacher(ctx, targetStr)
		if !ok {
			return sendArchiveAmbiguousOrNotFound(c, targetStr, candidates)
		}

		return sendTeacherArchiveSchedule(ctx, c, teacherSvc, userRepo, validatedName, targetDate)
	}
}

func sendArchiveWeeksForTarget(
	ctx context.Context,
	c tele.Context,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	targetStr string,
) error {
	if gID, err := strconv.Atoi(targetStr); err == nil && gID > 0 {
		weeks, err := schedSvc.GetAllWeeks(ctx)
		if err != nil || len(weeks) == 0 {
			return c.Send(fmt.Sprintf("Архив расписаний для группы %d пока пуст.", gID), keyboard.ReplyScheduleKeyboard)
		}
		msg := fmt.Sprintf("📦 *Архив расписания группы %d*\nВыберите неделю:", gID)
		kbd := keyboard.InlineArchiveWeeksKeyboard("g", strconv.Itoa(gID), weeks)
		return c.Send(msg, tele.ModeMarkdown, kbd)
	}

	validatedName, candidates, ok := teacherSvc.ValidateTeacher(ctx, targetStr)
	if !ok {
		return sendArchiveAmbiguousOrNotFound(c, targetStr, candidates)
	}

	weeks, err := teacherSvc.GetAllWeeks(ctx, validatedName)
	if err != nil || len(weeks) == 0 {
		return c.Send(fmt.Sprintf("Архив расписаний для преподавателя *%s* пока пуст.", formatter.EscapeMD(validatedName)), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}
	msg := fmt.Sprintf("📦 *Архив расписания: %s*\nВыберите неделю:", formatter.EscapeMD(validatedName))
	kbd := keyboard.InlineArchiveWeeksKeyboard("t", validatedName, weeks)
	return c.Send(msg, tele.ModeMarkdown, kbd)
}

func sendArchiveAmbiguousOrNotFound(c tele.Context, targetStr string, candidates []string) error {
	if len(candidates) > 0 {
		var sb strings.Builder
		fmt.Fprintf(&sb, "По запросу *%s* найдено несколько преподавателей:\n\n", formatter.EscapeMD(targetStr))
		for _, cand := range candidates {
			fmt.Fprintf(&sb, "• `/archive %s`\n", cand)
		}
		return c.Send(sb.String(), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}
	return c.Send(fmt.Sprintf("Преподаватель или группа *%s* не найдены в архиве.", formatter.EscapeMD(targetStr)), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
}

func sendGroupArchiveSchedule(ctx context.Context, c tele.Context, svc ScheduleService, groupID int32, subgroup int, date time.Time) error {
	group, err := svc.GetArchiveSchedule(ctx, groupID, date)
	if err != nil {
		return c.Send(
			fmt.Sprintf("Расписание группы %d за дату *%s* не найдено в архиве.", groupID, date.Format("02.01.2006")),
			tele.ModeMarkdown,
			keyboard.ReplyScheduleKeyboard,
		)
	}

	msg := formatter.FormatScheduleWeekForSubgroup(group, subgroup)
	return c.Send(msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
}

func sendTeacherArchiveSchedule(ctx context.Context, c tele.Context, svc TeacherService, userRepo UserRepo, teacherName string, date time.Time) error {
	teacher, err := svc.GetTeacherArchiveSchedule(ctx, teacherName, date)
	if err != nil {
		return c.Send(
			fmt.Sprintf("Расписание преподавателя *%s* за дату *%s* не найдено в архиве.", formatter.EscapeMD(teacherName), date.Format("02.01.2006")),
			tele.ModeMarkdown,
			keyboard.ReplyScheduleKeyboard,
		)
	}

	var profile string
	if shouldShowTeacherProfile(ctx, userRepo, c.Chat().ID, teacherName) {
		member, _ := svc.GetFacultyMember(ctx, teacherName)
		profile = formatter.FormatTeacherProfile(member)
	}
	msg := profile + formatter.FormatTeacherScheduleWeek(teacher)
	return c.Send(msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
}

func parseArchiveDate(s string) (time.Time, bool) {
	formats := []string{
		"02.01.2006",
		"2.1.2006",
		"2006-01-02",
		"02/01/2006",
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func handleArchiveNoArgs(
	ctx context.Context,
	c tele.Context,
	log *slog.Logger,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	userRepo UserRepo,
	sm StateManager,
) error {
	groupID, _, teacher, _ := userRepo.GetUserProfile(ctx, c.Chat().ID)
	if groupID > 0 {
		weeks, err := schedSvc.GetAllWeeks(ctx)
		if err != nil || len(weeks) == 0 {
			return c.Send("📦 Архив расписаний для групп пока пуст.", keyboard.ReplyScheduleKeyboard)
		}
		msg := fmt.Sprintf("📦 *Архив расписания группы %d*\nВыберите неделю для просмотра:", groupID)
		kbd := keyboard.InlineArchiveWeeksKeyboard("g", strconv.Itoa(groupID), weeks)
		return c.Send(msg, tele.ModeMarkdown, kbd)
	}

	if teacher != "" {
		weeks, err := teacherSvc.GetAllWeeks(ctx, teacher)
		if err != nil || len(weeks) == 0 {
			return c.Send(fmt.Sprintf("📦 Архив расписаний для преподавателя *%s* пока пуст.", formatter.EscapeMD(teacher)), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
		}
		msg := fmt.Sprintf("📦 *Архив расписания: %s*\nВыберите неделю для просмотра:", formatter.EscapeMD(teacher))
		kbd := keyboard.InlineArchiveWeeksKeyboard("t", teacher, weeks)
		return c.Send(msg, tele.ModeMarkdown, kbd)
	}

	if sm != nil {
		if err := sm.Set(c.Chat().ID, state.WaitingArchive); err != nil {
			log.Error("failed setting waiting archive state", "error", err)
			return c.Send(messages.Internal)
		}
	}
	return c.Send(messages.WaitingArchive)
}
