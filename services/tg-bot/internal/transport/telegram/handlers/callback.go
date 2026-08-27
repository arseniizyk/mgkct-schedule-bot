package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
)

func CallbacksHandler(log *slog.Logger, scheduleProvider ScheduleProvider, weekProvider WeekProvider, teacherScheduleProvider TeacherScheduleProvider, teacherWeekProvider TeacherWeekProvider) tele.HandlerFunc {
	return func(c tele.Context) error {
		callback := c.Callback()
		chatID := c.Chat().ID

		logger := log.With(
			"operation", "transport.telegram.handlers.Callback",
			"chat_id", chatID,
			"username", c.Chat().Username,
			"callback_data", callback.Data,
		)

		data := callback.Data
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var err error
		switch {
		case strings.Contains(data, keyboard.TeacherCurrentWeek):
			err = handleTeacherCurrentWeek(ctx, logger, c, data, teacherScheduleProvider, teacherWeekProvider)

		case strings.Contains(data, keyboard.TeacherPrevWeek), strings.Contains(data, keyboard.TeacherNextWeek):
			err = handleTeacherPrevNextWeek(ctx, logger, c, data, teacherScheduleProvider, teacherWeekProvider)

		case strings.Contains(data, keyboard.TeacherPrevDay), strings.Contains(data, keyboard.TeacherNextDay):
			err = handleTeacherDayNavigation(ctx, logger, c, data, teacherScheduleProvider)

		case strings.Contains(data, keyboard.CurrentWeek):
			err = handleCurrentWeek(ctx, logger, c, data, scheduleProvider, weekProvider)

		case strings.Contains(data, keyboard.PrevWeek), strings.Contains(data, keyboard.NextWeek):
			err = handlePrevNextWeek(ctx, logger, c, data, scheduleProvider, weekProvider)

		case strings.Contains(data, keyboard.PrevDay), strings.Contains(data, keyboard.NextDay):
			err = handleDayNavigation(ctx, logger, c, data, scheduleProvider)

		default:
			logger.Error("unexpected callback", "data", data)
			err = respondInternalError(c)
		}

		if err != nil {
			return err
		}

		// Answer the callback query exactly once on the success path.
		if err := c.Respond(); err != nil {
			logger.Warn("callback respond failed", "error", err)
		}
		return nil
	}
}

func handleCurrentWeek(ctx context.Context, log *slog.Logger, c tele.Context, data string, scheduleProvider ScheduleProvider, weekProvider WeekProvider) error {
	groupID, err := strconv.Atoi(dataFromCallbackData(data))
	if err != nil {
		log.ErrorContext(ctx, "invalid callback data", "error", err)
		return respondInternalError(c)
	}

	schedule, err := scheduleProvider.GetGroupSchedule(ctx, groupID)
	if err != nil {
		log.ErrorContext(ctx, "failed to get group schedule", "group_id", groupID, "error", err)
		return respondInternalError(c)
	}

	msg := formatter.FormatScheduleWeek(schedule)

	weeks, err := weekProvider.GetAvailableWeeks(ctx, nil)
	if err != nil {
		log.ErrorContext(ctx, "failed to get available weeks", "error", err)
		return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard, keyboard.InlineEmptyKeyboard)
	}
	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineWeekKeyboard(groupID, weeks))
}

func handlePrevNextWeek(ctx context.Context, log *slog.Logger, c tele.Context, data string, scheduleProvider ScheduleProvider, weekProvider WeekProvider) error {
	groupID, date, err := parseCallbackWeekNavigation(data)
	if err != nil {
		log.ErrorContext(ctx, "failed parsing callback week navigation data", "error", err)
		return respondInternalError(c)
	}

	schedule, err := scheduleProvider.GetGroupScheduleByWeek(ctx, groupID, date)
	if err != nil {
		log.ErrorContext(ctx, "failed to get schedule for week", "group_id", groupID, "date", date.String(), "error", err)
		return respondInternalError(c)
	}

	msg := formatter.FormatScheduleWeek(schedule)

	weeks, err := weekProvider.GetAvailableWeeks(ctx, &date)
	if err != nil {
		log.ErrorContext(ctx, "failed to get available weeks", "date", date, "error", err)
		return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard, keyboard.InlineEmptyKeyboard)
	}
	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineWeekKeyboard(groupID, weeks))
}

func handleDayNavigation(ctx context.Context, log *slog.Logger, c tele.Context, data string, scheduleProvider ScheduleProvider) error {
	groupID, dayIdx, err := parseCallbackDayNavigation(data)
	if err != nil {
		log.ErrorContext(ctx, "failed parsing day navigation callback data", "error", err)
		return respondInternalError(c)
	}

	schedule, err := scheduleProvider.GetGroupSchedule(ctx, groupID)
	if err != nil {
		log.ErrorContext(ctx, "failed to get schedule for day navigation", "group_id", groupID, "day_idx", dayIdx, "error", err)
		return respondInternalError(c)
	}

	msg, err := formatter.FormatScheduleDayAt(schedule, dayIdx)
	if err != nil {
		log.ErrorContext(ctx, "failed to format day", "day_idx", dayIdx, "error", err)
		return respondInternalError(c)
	}

	return c.Edit(msg, tele.ModeMarkdown, keyboard.InlineDayKeyboard(groupID, dayIdx, len(schedule.GetDays())))
}

func respondInternalError(c tele.Context) error {
	return c.Respond(&tele.CallbackResponse{
		Text:      messages.Internal,
		ShowAlert: true,
	})
}

func dataFromCallbackData(data string) string {
	parts := strings.Split(data, "|")
	if len(parts) > 1 {
		return parts[1]
	}

	return ""
}

func parseCallbackWeekNavigation(data string) (int, time.Time, error) {
	parsed := dataFromCallbackData(data)
	parts := strings.Split(parsed, ":")
	if len(parts) < 2 {
		return 0, time.Time{}, fmt.Errorf("failed splitting data by parts")
	}

	groupID, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed parsing group_id to int: %w", err)
	}

	date, err := time.Parse("02.01.2006", parts[1])
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed parsing text to date: %w", err)
	}

	return groupID, date, nil
}

// parseCallbackDayNavigation разбирает "<groupID>:<dayIdx>" из колбэка
// навигации по дням; dayIdx — индекс дня, в который ведёт стрелка (0 = Пн).
func parseCallbackDayNavigation(data string) (groupID, dayIdx int, err error) {
	parsed := dataFromCallbackData(data)

	parts := strings.Split(parsed, ":")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("failed splitting data by parts")
	}

	if groupID, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, fmt.Errorf("failed parsing group_id to int: %w", err)
	}

	if dayIdx, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, fmt.Errorf("failed parsing day index to int: %w", err)
	}

	return groupID, dayIdx, nil
}

func handleTeacherCurrentWeek(ctx context.Context, log *slog.Logger, c tele.Context, data string, teacherScheduleProvider TeacherScheduleProvider, teacherWeekProvider TeacherWeekProvider) error {
	chatID, err := chatIDFromCallbackData(data)
	if err != nil {
		log.ErrorContext(ctx, "invalid callback data", "error", err)
		return respondInternalError(c)
	}

	teacher, err := teacherScheduleProvider.GetTeacherScheduleByChatID(ctx, chatID)
	if err != nil {
		log.ErrorContext(ctx, "failed to get teacher schedule", "chat_id", chatID, "error", err)
		return respondInternalError(c)
	}

	msg := formatter.FormatTeacherScheduleWeek(teacher)

	weeks, err := teacherWeekProvider.GetAvailableTeacherWeeks(ctx, nil)
	if err != nil {
		log.ErrorContext(ctx, "failed to get available weeks", "error", err)
		return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard, keyboard.InlineEmptyKeyboard)
	}

	entitiesWeeks := convertToEntitiesWeeks(weeks)
	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineTeacherWeekKeyboard(chatID, entitiesWeeks))
}

func handleTeacherPrevNextWeek(ctx context.Context, log *slog.Logger, c tele.Context, data string, teacherScheduleProvider TeacherScheduleProvider, teacherWeekProvider TeacherWeekProvider) error {
	chatID, date, err := parseTeacherCallbackWeekNavigation(data)
	if err != nil {
		log.ErrorContext(ctx, "failed parsing callback week navigation data", "error", err)
		return respondInternalError(c)
	}

	teacherName, err := teacherNameByChatID(ctx, teacherScheduleProvider, chatID)
	if err != nil {
		log.ErrorContext(ctx, "failed to resolve teacher name", "chat_id", chatID, "error", err)
		return respondInternalError(c)
	}

	teacher, err := teacherScheduleProvider.GetTeacherScheduleByWeek(ctx, teacherName, date)
	if err != nil {
		log.ErrorContext(ctx, "failed to get teacher schedule for week", "name", teacherName, "date", date.String(), "error", err)
		return respondInternalError(c)
	}

	msg := formatter.FormatTeacherScheduleWeek(teacher)

	weeks, err := teacherWeekProvider.GetAvailableTeacherWeeks(ctx, &date)
	if err != nil {
		log.ErrorContext(ctx, "failed to get available weeks", "date", date, "error", err)
		return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard, keyboard.InlineEmptyKeyboard)
	}

	entitiesWeeks := convertToEntitiesWeeks(weeks)
	return editLongMessage(c, msg, tele.ModeMarkdown, keyboard.InlineTeacherWeekKeyboard(chatID, entitiesWeeks))
}

func handleTeacherDayNavigation(ctx context.Context, log *slog.Logger, c tele.Context, data string, teacherScheduleProvider TeacherScheduleProvider) error {
	chatID, dayIdx, err := parseTeacherCallbackDayNavigation(data)
	if err != nil {
		log.ErrorContext(ctx, "failed parsing day navigation callback data", "error", err)
		return respondInternalError(c)
	}

	teacher, err := teacherScheduleProvider.GetTeacherScheduleByChatID(ctx, chatID)
	if err != nil {
		log.ErrorContext(ctx, "failed to get teacher schedule for day navigation", "chat_id", chatID, "day_idx", dayIdx, "error", err)
		return respondInternalError(c)
	}

	msg, err := formatter.FormatTeacherScheduleDayAt(teacher, dayIdx)
	if err != nil {
		log.ErrorContext(ctx, "failed to format day", "day_idx", dayIdx, "error", err)
		return respondInternalError(c)
	}

	return c.Edit(msg, tele.ModeMarkdown, keyboard.InlineTeacherDayKeyboard(chatID, dayIdx, len(teacher.GetDays())))
}

func teacherNameByChatID(ctx context.Context, provider TeacherScheduleProvider, chatID int64) (string, error) {
	teacher, err := provider.GetTeacherScheduleByChatID(ctx, chatID)
	if err != nil {
		return "", err
	}
	return teacher.GetName(), nil
}

func chatIDFromCallbackData(data string) (int64, error) {
	raw := dataFromCallbackData(data)
	chatID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed parsing chat_id: %w", err)
	}
	return chatID, nil
}

func parseTeacherCallbackWeekNavigation(data string) (int64, time.Time, error) {
	parsed := dataFromCallbackData(data)
	parts := strings.Split(parsed, ":")
	if len(parts) < 2 {
		return 0, time.Time{}, fmt.Errorf("failed splitting data by parts")
	}

	chatID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed parsing chat_id to int: %w", err)
	}

	date, err := time.Parse("02.01.2006", parts[1])
	if err != nil {
		return 0, time.Time{}, fmt.Errorf("failed parsing text to date: %w", err)
	}

	return chatID, date, nil
}

func parseTeacherCallbackDayNavigation(data string) (chatID int64, dayIdx int, err error) {
	parsed := dataFromCallbackData(data)

	parts := strings.Split(parsed, ":")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("failed splitting data by parts")
	}

	if chatID, err = strconv.ParseInt(parts[0], 10, 64); err != nil {
		return 0, 0, fmt.Errorf("failed parsing chat_id to int: %w", err)
	}

	if dayIdx, err = strconv.Atoi(parts[1]); err != nil {
		return 0, 0, fmt.Errorf("failed parsing day index to int: %w", err)
	}

	return chatID, dayIdx, nil
}
