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

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if strings.Contains(data, keyboard.BroadcastConfirm) || strings.Contains(data, keyboard.BroadcastCancel) {
			return HandleBroadcastCallback(ctx, log, c, data, userRepo, bot, sm, adminIDs)
		}

		err := dispatchCallback(ctx, log, c, data, schedSvc, teacherSvc, analyticsSvc, userRepo, sm, adminIDs, dict)
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
	data string,
	schedSvc ScheduleService,
	teacherSvc TeacherService,
	analyticsSvc AnalyticsService,
	userRepo UserRepo,
	sm StateManager,
	adminIDs []int64,
	dict calendar.SubjectDictionary,
) error {
	switch {
	// Выбор роли
	case strings.Contains(data, keyboard.RoleSelect):
		if sm != nil {
			return handleRoleSelect(ctx, log, c, data, sm)
		}

	// Уведомления
	case strings.Contains(data, keyboard.NotifyToggle):
		if userRepo != nil {
			return handleNotifyToggle(ctx, log, c, data, userRepo)
		}

	// Выбор подгруппы
	case strings.Contains(data, keyboard.SubgroupSelect):
		if userRepo != nil {
			return handleSubgroupSelect(ctx, log, c, data, userRepo, sm)
		}

	// Кабинеты
	case strings.Contains(data, keyboard.CabinetPrevDay), strings.Contains(data, keyboard.CabinetNextDay):
		if analyticsSvc != nil {
			return handleCabinetDayNav(ctx, log, c, data, analyticsSvc)
		}
	case strings.Contains(data, keyboard.CabinetCurrentWeek):
		if analyticsSvc != nil {
			return handleCabinetWeek(ctx, log, c, data, analyticsSvc)
		}

	// Архив
	case strings.Contains(data, keyboard.ArchiveBack):
		return handleArchiveBack(ctx, log, c, data, schedSvc, teacherSvc)
	case strings.Contains(data, keyboard.ArchiveSelect):
		return handleArchiveSelect(ctx, log, c, data, schedSvc, teacherSvc, userRepo)

	// Закрепление преподавателя
	case strings.Contains(data, keyboard.TeacherSetSelect):
		if userRepo != nil {
			return handleTeacherSetSelect(ctx, log, c, data, userRepo, sm)
		}

	// Преподаватели (ОБЯЗАТЕЛЬНО перед группами, чтобы t_currweek/t_prevday не попадали в обработчик групп!)
	case strings.Contains(data, keyboard.TeacherWeekCalendar):
		return handleTeacherWeekCalendar(ctx, log, c, data, teacherSvc, dict)
	case strings.Contains(data, keyboard.TeacherWeekImage):
		return handleTeacherWeekImage(ctx, log, c, data, teacherSvc)
	case strings.Contains(data, keyboard.TeacherPrevDay), strings.Contains(data, keyboard.TeacherNextDay):
		return handleTeacherDayNav(ctx, log, c, data, teacherSvc, userRepo)
	case strings.Contains(data, keyboard.TeacherCurrentWeek):
		return handleTeacherCurrentWeek(ctx, log, c, data, teacherSvc, userRepo)
	case strings.Contains(data, keyboard.TeacherPrevWeek), strings.Contains(data, keyboard.TeacherNextWeek):
		return handleTeacherWeekNav(ctx, log, c, data, teacherSvc, userRepo)

	// Группы
	case strings.Contains(data, keyboard.WeekCalendar):
		return handleWeekCalendar(ctx, log, c, data, schedSvc, userRepo, dict)
	case strings.Contains(data, keyboard.WeekImage):
		return handleWeekImage(ctx, log, c, data, schedSvc, userRepo)
	case strings.Contains(data, keyboard.PrevDay), strings.Contains(data, keyboard.NextDay),
		strings.Contains(data, keyboard.DaySubgroup),
		strings.Contains(data, "prevday"), strings.Contains(data, "nextday"):
		return handleDayNav(ctx, log, c, data, schedSvc, userRepo)
	case strings.Contains(data, keyboard.WeekSubgroup):
		return handleWeekSubgroup(ctx, log, c, data, schedSvc, userRepo)
	case strings.Contains(data, keyboard.CurrentWeek), strings.Contains(data, "currentweek"):
		return handleCurrentWeek(ctx, log, c, data, schedSvc, userRepo)
	case strings.Contains(data, keyboard.PrevWeek), strings.Contains(data, keyboard.NextWeek),
		strings.Contains(data, "prevweek"), strings.Contains(data, "nextweek"):
		return handleWeekNav(ctx, log, c, data, schedSvc, userRepo)

	default:
		log.Warn("unexpected callback", "data", data)
		return c.Respond(&tele.CallbackResponse{Text: messages.Internal, ShowAlert: true})
	}

	return nil
}
