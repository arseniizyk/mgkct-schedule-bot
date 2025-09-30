package bot

import (
	"context"
	"errors"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/bot/formatter"
	kbd "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/bot/keyboard"
	msg "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/bot/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/bot/utils"
	tele "gopkg.in/telebot.v4"
)

func (h *Handler) Week(c tele.Context) error {
	schedule, msg := h.fetchSchedule(c, nil)
	if msg != "" {
		return c.Send(msg)
	}
	return c.Send(formatter.FormatScheduleWeek(schedule), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard)
}

func (h *Handler) Day(c tele.Context) error {
	schedule, msg := h.fetchSchedule(c, nil)
	if msg != "" {
		return c.Send(msg)
	}
	return h.handleEndTime(c, schedule)
}

func (h *Handler) SendUpdate(chatID int64, group *pb.Group) error {
	msg := "*Расписание обновлено*\n\n"
	msg += formatter.FormatScheduleWeek(group)
	slog.Info("Send Updated Schedule", "chat_id", chatID, "group_id", group.Id)

	_, err := h.bot.Send(tele.ChatID(chatID), msg, tele.ModeMarkdown)
	return err
}

func (h *Handler) Calls(c tele.Context) error {
	return c.Send(msg.Calls, tele.ModeMarkdown)
}

func (h *Handler) fetchSchedule(c tele.Context, groupID *int) (*pb.Group, string) {
	group, err := h.getGroupSchedule(c, groupID)
	if err != nil {
		switch {
		case errors.Is(err, models.ErrGroupNotFound):
			return nil, msg.GroupNotFound
		case errors.Is(err, models.ErrUserNoGroup):
			return nil, msg.UserNoGroup
		case errors.Is(err, models.ErrBadInput):
			return nil, msg.OnlyNumbers
		default:
			return nil, msg.Internal
		}
	}
	return group, ""
}

func (h *Handler) getGroupSchedule(c tele.Context, groupID *int) (*pb.Group, error) {
	var (
		groupNum int
		err      error
	)

	if groupID != nil {
		groupNum = *groupID
	} else {
		groupNum, err = utils.InputNum(c)
		if err != nil {
			slog.Warn("getGroupSchedule: can't parse input to int", "input", c.Args()[0], "err", err)
			return nil, err
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if groupNum != 0 {
		group, err := h.telegramService.GetGroupSchedule(ctx, groupNum)
		if err != nil {
			slog.Warn("getGroupSchedule: failed by id", "chat_id", c.Chat().ID, "group_id", groupNum, "err", err)
			return nil, err
		}
		return group, nil
	}

	group, err := h.telegramService.GetGroupScheduleByChatID(ctx, c.Chat().ID)
	if err != nil {
		slog.Warn("getGroupSchedule:", "chat_id", c.Chat().ID, "err", err)
		return nil, err
	}

	return group, nil
}

func (h *Handler) handleEndTime(c tele.Context, group *pb.Group) error {
	dayIdx := utils.Day()
	day := group.Days[dayIdx]

	lastSubject := utils.FindLastSubject(day.Subjects)
	if lastSubject == -1 { // if no pairs in day
		return c.Send(formatter.FormatScheduleDay(group.Days[utils.Day(1)]), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard, kbd.InlineScheduleKeyboard(int(group.Id)))
	}

	now := time.Now()

	endTime, ok := utils.GetEndTime(dayIdx, lastSubject)
	if ok {
		if now.After(endTime) || now.Equal(endTime) {
			return c.Send(formatter.FormatScheduleDay(group.Days[utils.Day(1)]), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard, kbd.InlineScheduleKeyboard(int(group.Id)))
		}
	}

	return c.Send(formatter.FormatScheduleDay(group.Days[dayIdx]), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard, kbd.InlineScheduleKeyboard(int(group.Id)))
}
