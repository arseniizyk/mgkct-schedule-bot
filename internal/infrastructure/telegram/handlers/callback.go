package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/calendar"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/botutil"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
)

func CallbacksHandler(
	log *slog.Logger,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	analyticsSvc AnalyticsService,
	userRepo UserRepo,
	sm StateManager,
	extra ...any,
) tele.HandlerFunc {
	var bot BotSender
	var adminIDs []int64
	var dict calendar.SubjectDictionary
	for _, ex := range extra {
		switch v := ex.(type) {
		case BotSender:
			bot = v
		case []int64:
			adminIDs = v
		case calendar.SubjectDictionary:
			dict = v
		case context.Context:
			SetBroadcastContext(v)
		}
	}

	return func(c tele.Context) error {
		callback := c.Callback()
		data := callback.Data
		unique := extractCallbackUnique(data)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if unique == keyboard.BroadcastConfirm || unique == keyboard.BroadcastCancel {
			return HandleBroadcastCallback(ctx, log, c, data, userRepo, bot, sm, adminIDs)
		}

		err := dispatchCallback(ctx, log, c, unique, data, schedSvc, teacherSvc, analyticsSvc, userRepo, sm, dict)
		if err != nil {
			if errors.Is(err, tele.ErrTrueResult) || strings.Contains(err.Error(), "result is True") || strings.Contains(err.Error(), "message is not modified") {
				return c.Respond()
			}
			if retryAfter, ok := botutil.ExtractFloodRetry(err); ok {
				log.Warn("rate limited by telegram (flood control)", "retry_after", retryAfter, "data", data)
				text := "⏳ Слишком частые переключения. Подождите немного."
				if retryAfter > 0 {
					text = fmt.Sprintf("⏳ Слишком частые переключения. Подождите %d сек.", retryAfter)
				}
				return c.Respond(&tele.CallbackResponse{Text: text, ShowAlert: false})
			}
			log.Error("callback execution error", "error", err, "data", data)
			return c.Respond(&tele.CallbackResponse{Text: messages.Internal, ShowAlert: true})
		}

		if err := c.Respond(); err != nil {
			if strings.Contains(err.Error(), "query is too old") ||
				strings.Contains(err.Error(), "QUERY_ID_INVALID") ||
				strings.Contains(err.Error(), "query ID is invalid") {
				return nil
			}
			return err
		}
		return nil
	}
}

func extractCallbackUnique(data string) string {
	head, _, _ := strings.Cut(data, "|")
	return strings.TrimSpace(strings.TrimPrefix(head, "\f"))
}

func callbackChatID(c tele.Context) int64 {
	if c.Chat() != nil {
		return c.Chat().ID
	}
	if c.Sender() != nil {
		return c.Sender().ID
	}
	return 0
}

func dispatchCallback(
	ctx context.Context,
	log *slog.Logger,
	c tele.Context,
	unique, data string,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	analyticsSvc AnalyticsService,
	userRepo UserRepo,
	sm StateManager,
	dict calendar.SubjectDictionary,
) error {
	switch unique {
	// Выбор роли
	case keyboard.RoleSelect:
		if sm != nil {
			return handleRoleSelect(ctx, log, c, data, sm)
		}

	// Уведомления
	case keyboard.NotifyToggle:
		if userRepo != nil {
			return handleNotifyToggle(ctx, log, c, data, userRepo)
		}

	// Выбор подгруппы
	case keyboard.SubgroupSelect:
		if userRepo != nil {
			return handleSubgroupSelect(ctx, log, c, data, userRepo, sm)
		}

	// Кабинеты
	case keyboard.CabinetPrevDay, keyboard.CabinetNextDay:
		if analyticsSvc != nil {
			return handleCabinetDayNav(ctx, log, c, data, analyticsSvc)
		}
	case keyboard.CabinetCurrentWeek:
		if analyticsSvc != nil {
			return handleCabinetWeek(ctx, log, c, data, analyticsSvc)
		}

	// Архив
	case keyboard.ArchiveBack:
		return handleArchiveBack(ctx, log, c, data, schedSvc, teacherSvc)
	case keyboard.ArchiveSelect:
		return handleArchiveSelect(ctx, log, c, data, schedSvc, teacherSvc, userRepo)

	// Закрепление преподавателя
	case keyboard.TeacherSetSelect:
		if userRepo != nil {
			return handleTeacherSetSelect(ctx, log, c, data, userRepo, sm)
		}

	// Преподаватели
	case keyboard.TeacherWeekCalendar:
		return handleTeacherWeekCalendar(ctx, log, c, data, teacherSvc, dict)
	case keyboard.TeacherWeekImage:
		return handleTeacherWeekImage(ctx, log, c, data, teacherSvc)
	case keyboard.TeacherPrevDay, keyboard.TeacherNextDay:
		return handleTeacherDayNav(ctx, log, c, data, teacherSvc, userRepo)
	case keyboard.TeacherCurrentWeek:
		return handleTeacherCurrentWeek(ctx, log, c, data, teacherSvc, userRepo)
	case keyboard.TeacherPrevWeek, keyboard.TeacherNextWeek:
		return handleTeacherWeekNav(ctx, log, c, data, teacherSvc, userRepo)

	// Группы
	case keyboard.WeekCalendar:
		return handleWeekCalendar(ctx, log, c, data, schedSvc, userRepo, dict)
	case keyboard.WeekImage:
		return handleWeekImage(ctx, log, c, data, schedSvc, userRepo)
	case keyboard.PrevDay, keyboard.NextDay, keyboard.DaySubgroup:
		return handleDayNav(ctx, log, c, data, schedSvc, userRepo)
	case keyboard.WeekSubgroup:
		return handleWeekSubgroup(ctx, log, c, data, schedSvc, userRepo)
	case keyboard.CurrentWeek:
		return handleCurrentWeek(ctx, log, c, data, schedSvc, userRepo)
	case keyboard.PrevWeek, keyboard.NextWeek:
		return handleWeekNav(ctx, log, c, data, schedSvc, userRepo)

	default:
		log.Warn("unexpected callback", "unique", unique, "data", data)
		return c.Respond(&tele.CallbackResponse{Text: messages.Internal, ShowAlert: true})
	}

	return nil
}
