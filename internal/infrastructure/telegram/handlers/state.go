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
)

func StatesHandler(
	log *slog.Logger,
	userRepo UserRepo,
	sm StateManager,
	teacherSvc TeacherService,
	schedSvc ScheduleService,
	analyticsSvc AnalyticsService,
	subjectDict SubjectDict,
	_ BotSender,
	adminIDs []int64,
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
			return handleWaitingGroup(ctx, log, c, schedSvc, userRepo, sm)
		case state.WaitingTeacher:
			return handleWaitingTeacher(ctx, log, c, userRepo, sm, teacherSvc)
		case state.WaitingTeacherSchedule:
			return handleWaitingTeacherSchedule(ctx, log, c, userRepo, sm, teacherSvc)
		case state.WaitingTeacherDay:
			return handleWaitingTeacherDay(ctx, log, c, userRepo, sm, teacherSvc)
		case state.WaitingCabinet:
			return handleWaitingCabinet(ctx, log, c, sm, analyticsSvc)
		case state.WaitingSubject:
			return handleWaitingSubject(c, sm, subjectDict)
		case state.WaitingArchive:
			return handleWaitingArchive(ctx, c, sm, schedSvc, teacherSvc)
		case state.WaitingBroadcast:
			if !IsAdmin(adminIDs, chatID) {
				_ = sm.Clear(chatID)
				return c.Send(messages.Cancelled)
			}
			return handleWaitingBroadcast(ctx, log, c, sm)
		default:
			_ = sm.Clear(chatID)
			return c.Send(messages.Cancelled)
		}
	}
}

func handleWaitingGroup(ctx context.Context, log *slog.Logger, c tele.Context, schedSvc ScheduleService, userRepo UserRepo, sm StateManager) error {
	chatID := c.Chat().ID
	fields := strings.Fields(c.Text())
	if len(fields) == 0 {
		return c.Send(messages.OnlyNumbers)
	}

	groupID, err := strconv.Atoi(fields[0])
	if err != nil || groupID <= 0 {
		return c.Send(messages.OnlyNumbers)
	}

	if schedSvc != nil {
		exists, err := schedSvc.HasGroup(ctx, int32(groupID))
		if err != nil {
			log.Error("failed checking group existence in handleWaitingGroup", "group_id", groupID, "error", err)
			return c.Send(messages.Internal)
		}
		if !exists {
			return c.Send(fmt.Sprintf("%s. %s", messages.GroupNotFound, messages.WaitingGroup))
		}
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
		return sendTeacherSetNotFoundOrSuggestions(ctx, c, teacherSvc, teacherQuery, candidates)
	}

	if err := userRepo.SetTeacher(ctx, chatID, matched); err != nil {
		log.Error("failed saving teacher from state", "name", matched, "error", err)
		return c.Send(messages.Internal)
	}

	_ = sm.Clear(chatID)
	return c.Send(messages.TeacherSaved, keyboard.ReplyScheduleKeyboard)
}

func handleWaitingTeacherSchedule(ctx context.Context, log *slog.Logger, c tele.Context, userRepo UserRepo, sm StateManager, teacherSvc TeacherService) error {
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

	var profile string
	if shouldShowTeacherProfile(ctx, userRepo, chatID, matched) {
		member, _ := teacherSvc.GetFacultyMember(ctx, matched)
		profile = formatter.FormatTeacherProfile(member)
	}
	msg := profile + formatter.FormatTeacherScheduleWeek(teacher)
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

func handleWaitingTeacherDay(ctx context.Context, log *slog.Logger, c tele.Context, userRepo UserRepo, sm StateManager, teacherSvc TeacherService) error {
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

	var profile string
	if shouldShowTeacherProfile(ctx, userRepo, chatID, matched) {
		member, _ := teacherSvc.GetFacultyMember(ctx, matched)
		profile = formatter.FormatTeacherProfile(member)
	}
	msg := profile + formatter.FormatTeacherScheduleDay(teacher)
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

func handleWaitingCabinet(ctx context.Context, log *slog.Logger, c tele.Context, sm StateManager, analyticsSvc AnalyticsService) error {
	_ = sm.Clear(c.Chat().ID)
	return sendCabinetScheduleByQuery(ctx, log, c, analyticsSvc, strings.TrimSpace(c.Text()))
}

func handleWaitingSubject(c tele.Context, sm StateManager, subjectDict SubjectDict) error {
	_ = sm.Clear(c.Chat().ID)
	return sendSubjectByQuery(c, subjectDict, strings.TrimSpace(c.Text()))
}

func handleWaitingArchive(ctx context.Context, c tele.Context, sm StateManager, schedSvc ScheduleService, teacherSvc TeacherService) error {
	_ = sm.Clear(c.Chat().ID)
	return sendArchiveWeeksForTarget(ctx, c, schedSvc, teacherSvc, strings.TrimSpace(c.Text()))
}
