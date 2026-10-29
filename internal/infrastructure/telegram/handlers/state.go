package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

func StatesHandler(
	log *slog.Logger,
	userRepo UserRepo,
	sm StateManager,
	teacherSvc TeacherService,
	schedSvc ScheduleService,
	analyticsSvc AnalyticsService,
	subjectDict SubjectDict,
) tele.HandlerFunc {
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
			return handleWaitingGroup(ctx, log, c, userRepo, sm)
		case state.WaitingTeacher:
			return handleWaitingTeacher(ctx, log, c, userRepo, sm, teacherSvc)
		case state.WaitingTeacherSchedule:
			return handleWaitingTeacherSchedule(ctx, log, c, sm, teacherSvc)
		case state.WaitingTeacherDay:
			return handleWaitingTeacherDay(ctx, log, c, sm, teacherSvc)
		case state.WaitingCabinet:
			return handleWaitingCabinet(ctx, log, c, sm, analyticsSvc)
		case state.WaitingSubject:
			return handleWaitingSubject(c, sm, subjectDict)
		case state.WaitingArchive:
			return handleWaitingArchive(ctx, c, sm, schedSvc, teacherSvc)
		default:
			_ = sm.Clear(chatID)
			return c.Send(messages.Cancelled)
		}
	}
}

func handleWaitingGroup(ctx context.Context, log *slog.Logger, c tele.Context, userRepo UserRepo, sm StateManager) error {
	chatID := c.Chat().ID
	fields := strings.Fields(c.Text())
	if len(fields) == 0 {
		return c.Send(messages.OnlyNumbers)
	}

	groupID, err := strconv.Atoi(fields[0])
	if err != nil || groupID <= 0 {
		return c.Send(messages.OnlyNumbers)
	}

	if len(fields) >= 2 {
		subgroup, err := strconv.Atoi(fields[1])
		if err == nil && (subgroup == 1 || subgroup == 2 || subgroup == 0) {
			if err := userRepo.SetUserGroup(ctx, chatID, groupID, subgroup); err != nil {
				log.Error("failed saving group from state", "group_id", groupID, "error", err)
				return c.Send(messages.InternalTryWith)
			}
			_ = sm.Clear(chatID)
			if subgroup > 0 {
				return c.Send(fmt.Sprintf(messages.GroupWithSubgroupSaved, groupID, subgroup), keyboard.ReplyScheduleKeyboard)
			}
			return c.Send(messages.GroupSaved, keyboard.ReplyScheduleKeyboard)
		}
	}

	_ = sm.Clear(chatID)
	return c.Send(fmt.Sprintf(messages.SelectSubgroup, groupID), keyboard.InlineSubgroupKeyboard(groupID))
}

func handleWaitingTeacher(ctx context.Context, log *slog.Logger, c tele.Context, userRepo UserRepo, sm StateManager, teacherSvc TeacherService) error {
	chatID := c.Chat().ID
	teacherQuery := c.Text()
	matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, teacherQuery)
	if !ok {
		return sendTeacherNotFoundOrSuggestions(ctx, c, teacherSvc, teacherQuery, candidates)
	}

	if err := userRepo.SetTeacher(ctx, chatID, matched); err != nil {
		log.Error("failed saving teacher from state", "name", matched, "error", err)
		return c.Send(messages.Internal)
	}

	_ = sm.Clear(chatID)
	return c.Send(messages.TeacherSaved, keyboard.ReplyScheduleKeyboard)
}

func handleWaitingTeacherSchedule(ctx context.Context, log *slog.Logger, c tele.Context, sm StateManager, teacherSvc TeacherService) error {
	chatID := c.Chat().ID
	teacherQuery := c.Text()
	matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, teacherQuery)
	if !ok {
		return sendTeacherNotFoundOrSuggestions(ctx, c, teacherSvc, teacherQuery, candidates)
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
}

func handleWaitingTeacherDay(ctx context.Context, log *slog.Logger, c tele.Context, sm StateManager, teacherSvc TeacherService) error {
	chatID := c.Chat().ID
	teacherQuery := c.Text()
	matched, candidates, ok := teacherSvc.ValidateTeacher(ctx, teacherQuery)
	if !ok {
		return sendTeacherNotFoundOrSuggestions(ctx, c, teacherSvc, teacherQuery, candidates)
	}

	_ = sm.Clear(chatID)

	teacher, err := teacherSvc.GetTeacherLatestSchedule(ctx, matched)
	if err != nil {
		log.Warn("failed getting teacher day schedule from state", "name", matched, "error", err)
		return c.Send(formatter.FormatErrorMessage(err))
	}

	member, _ := teacherSvc.GetFacultyMember(ctx, matched)
	msg := formatter.FormatTeacherProfile(member) + formatter.FormatTeacherScheduleDay(teacher)
	dayIdx := formatter.EffectiveTeacherDayIndex(teacher)

	return c.Send(
		msg,
		tele.ModeMarkdown,
		keyboard.ReplyScheduleKeyboard,
		keyboard.InlineTeacherDayKeyboard(teacher.Name, dayIdx, len(teacher.Days)),
	)
}

func handleWaitingCabinet(ctx context.Context, log *slog.Logger, c tele.Context, sm StateManager, analyticsSvc AnalyticsService) error {
	chatID := c.Chat().ID
	query := strings.TrimSpace(c.Text())
	_ = sm.Clear(chatID)

	cabSched, candidates, err := analyticsSvc.GetCabinetSchedule(ctx, query, time.Time{})
	if err != nil {
		log.Warn("failed getting cabinet schedule from state", "query", query, "error", err)
		return c.Send("Ошибка при получении расписания кабинета.", keyboard.ReplyScheduleKeyboard)
	}

	if cabSched == nil {
		if len(candidates) > 0 {
			var sb strings.Builder
			fmt.Fprintf(&sb, "По запросу *%s* найдено несколько кабинетов:\n\n", query)
			for _, cand := range candidates {
				fmt.Fprintf(&sb, "• `/cabinet %s`\n", cand)
			}
			return c.Send(sb.String(), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
		}
		return c.Send(fmt.Sprintf("Кабинет *%s* не найден в расписании.", query), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}

	dayIdx := service.EffectiveEndingsDayIndex()
	if dayIdx >= len(cabSched.Days) {
		dayIdx = 0
	}

	msg := formatter.FormatCabinetDay(cabSched, dayIdx)
	return c.Send(
		msg,
		tele.ModeMarkdown,
		keyboard.ReplyScheduleKeyboard,
		keyboard.InlineCabinetDayKeyboard(cabSched.Cabinet, dayIdx, len(cabSched.Days)),
	)
}

func handleWaitingSubject(c tele.Context, sm StateManager, subjectDict SubjectDict) error {
	chatID := c.Chat().ID
	query := strings.TrimSpace(c.Text())
	_ = sm.Clear(chatID)

	cleaned, fullName, found := subjectDict.Resolve(query)
	if found {
		return c.Send(
			fmt.Sprintf("📖 *%s*\n%s", cleaned, fullName),
			tele.ModeMarkdown,
			keyboard.ReplyScheduleKeyboard,
		)
	}

	similar := subjectDict.Search(query)
	if len(similar) > 0 {
		var sb strings.Builder
		fmt.Fprintf(&sb, "Предмет *%s* не найден. Возможно, вы имели в виду:\n\n", query)
		limit := min(len(similar), 8)
		for i := range limit {
			fmt.Fprintf(&sb, "• *%s* — %s\n", similar[i].ShortName, similar[i].FullName)
		}
		return c.Send(sb.String(), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}

	return c.Send(
		fmt.Sprintf("Предмет *%s* не найден в справочнике колледжа.", query),
		tele.ModeMarkdown,
		keyboard.ReplyScheduleKeyboard,
	)
}

func handleWaitingArchive(ctx context.Context, c tele.Context, sm StateManager, schedSvc ScheduleService, teacherSvc TeacherService) error {
	chatID := c.Chat().ID
	targetStr := strings.TrimSpace(c.Text())
	_ = sm.Clear(chatID)

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
		if len(candidates) > 0 {
			var sb strings.Builder
			fmt.Fprintf(&sb, "По запросу *%s* найдено несколько преподавателей:\n\n", targetStr)
			for _, cand := range candidates {
				fmt.Fprintf(&sb, "• `/archive %s`\n", cand)
			}
			return c.Send(sb.String(), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
		}
		return c.Send(fmt.Sprintf("Преподаватель или группа *%s* не найдены в архиве.", targetStr), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}

	weeks, err := teacherSvc.GetAllWeeks(ctx, validatedName)
	if err != nil || len(weeks) == 0 {
		return c.Send(fmt.Sprintf("Архив расписаний для преподавателя *%s* пока пуст.", validatedName), tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard)
	}
	msg := fmt.Sprintf("📦 *Архив расписания: %s*\nВыберите неделю:", validatedName)
	kbd := keyboard.InlineArchiveWeeksKeyboard("t", validatedName, weeks)
	return c.Send(msg, tele.ModeMarkdown, kbd)
}
