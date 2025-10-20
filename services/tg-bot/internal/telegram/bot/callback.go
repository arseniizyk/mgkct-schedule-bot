package bot

import (
	"context"
	"log/slog"
	"strconv"

	kbd "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/bot/keyboard"
	msg "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/bot/messages"
	tele "gopkg.in/telebot.v4"
)

func (h *Handler) callbackCurrentWeek(c tele.Context) error {
	callback := c.Callback()
	groupID, err := strconv.Atoi(parseCallbackData(callback.Data))
	if err != nil {
		slog.Warn("callback: invalid data", "data", callback.Data, "chat_id", c.Chat().ID, "err", err)
		return c.Respond(&tele.CallbackResponse{Text: msg.Internal, ShowAlert: true})
	}

	schedule, msg := h.fetchSchedule(c, &groupID)
	if msg != "" {
		return c.Send(msg)
	}

	weeks, err := h.telegramService.GetAvailableWeeks(context.Background(), nil)
	if err != nil {
		slog.Error("can't get available weeks", "err", err)
		return c.Edit(formatScheduleWeek(schedule), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard, kbd.InlineEmptyKeyboard)
	}
	return c.Edit(formatScheduleWeek(schedule), tele.ModeMarkdown, kbd.InlineWeekKeyboard(groupID, weeks))

}

func (h *Handler) callbackWeekNavigation(c tele.Context) error {
	groupID, date, err := parseCallbackWeekNavigation(c.Callback())
	if err != nil {
		slog.Error("Failed parsing callback week navigation data", "err", err)
	}

	schedule, err := h.telegramService.GetGroupScheduleByWeek(context.Background(), groupID, date)
	if err != nil {
		slog.Warn("can't get schedule for week", "group_id", groupID, "date", date, "err", err)
		return c.Respond(&tele.CallbackResponse{Text: msg.Internal, ShowAlert: true})
	}

	weeks, err := h.telegramService.GetAvailableWeeks(context.Background(), &date)
	if err != nil {
		slog.Error("can't get available weeks", "date", date, "err", err)
		return c.Edit(formatScheduleWeek(schedule), tele.ModeMarkdown)
	}

	return c.Edit(formatScheduleWeek(schedule), tele.ModeMarkdown, kbd.InlineWeekKeyboard(groupID, weeks))
}
