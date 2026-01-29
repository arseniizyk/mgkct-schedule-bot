package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
	tele "gopkg.in/telebot.v4"
)

func CallbacksHandler(log *slog.Logger, scheduleProvider ScheduleProvider, weekProvider WeekProvider) tele.HandlerFunc {
	return func(c tele.Context) error {
		callback := c.Callback()
		chatID := c.Chat().ID

		logger := log.With(
			"operation", "transport.telegram.handlers.Callback",
			"chat_id", chatID,
			"username", c.Chat().Username,
			"callback_data", callback.Data,
		)

		if err := c.Respond(); err != nil {
			logger.Warn("callback respond failed", "error", err)
		}

		data := callback.Data
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		switch {
		case strings.Contains(data, keyboard.CurrentWeek):
			return handleCurrentWeek(ctx, logger, c, data, scheduleProvider, weekProvider)

		case strings.Contains(data, keyboard.PrevWeek), strings.Contains(data, keyboard.NextWeek):
			return handlePrevNextWeek(ctx, logger, c, data, scheduleProvider, weekProvider)

		default:
			logger.Error("unexpected callback", "data", data)
			return respondInternalError(c)
		}
	}
}

func handleCurrentWeek(ctx context.Context, log *slog.Logger, c tele.Context, data string, scheduleProvider ScheduleProvider, weekProvider WeekProvider) error {
	groupID, err := strconv.Atoi(dataFromCallbackData(data))
	if err != nil {
		log.Error("invalid callback data", "error", err)
		return respondInternalError(c)
	}

	schedule, err := scheduleProvider.GetGroupSchedule(ctx, groupID)
	if err != nil {
		log.Error("failed to get group schedule", "group_id", groupID, "error", err)
		return respondInternalError(c)
	}

	msg := formatter.FormatScheduleWeek(schedule)

	weeks, err := weekProvider.GetAvailableWeeks(ctx, nil)
	if err != nil {
		log.Error("failed to get available weeks", "error", err)
		return c.Edit(msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard, keyboard.InlineEmptyKeyboard)
	}
	return c.Edit(msg, tele.ModeMarkdown, keyboard.InlineWeekKeyboard(groupID, weeks))
}

func handlePrevNextWeek(ctx context.Context, log *slog.Logger, c tele.Context, data string, scheduleProvider ScheduleProvider, weekProvider WeekProvider) error {
	groupID, date, err := parseCallbackWeekNavigation(data)
	if err != nil {
		log.Error("failed parsing callback week navigation data", "error", err)
		return respondInternalError(c)
	}

	schedule, err := scheduleProvider.GetGroupScheduleByWeek(ctx, groupID, date)
	if err != nil {
		log.Error("failed to get schedule for week", "group_id", groupID, "date", date.String(), "error", err)
		return respondInternalError(c)
	}

	msg := formatter.FormatScheduleWeek(schedule)

	weeks, err := weekProvider.GetAvailableWeeks(ctx, &date)
	if err != nil {
		log.Error("failed to get available weeks", "date", date, "error", err)
		return c.Edit(msg, tele.ModeMarkdown, keyboard.ReplyScheduleKeyboard, keyboard.InlineEmptyKeyboard)
	}
	return c.Edit(msg, tele.ModeMarkdown, keyboard.InlineWeekKeyboard(groupID, weeks))
}

func respondInternalError(c tele.Context) error {
	return c.Respond(&tele.CallbackResponse{
		Text:      messages.Internal,
		ShowAlert: true,
	})
}

func dataFromCallbackData(data string) string {
	parts := strings.Split(data, "|")
	if len(parts) < 2 {
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
