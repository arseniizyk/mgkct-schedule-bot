package delivery

import (
	"context"
	"errors"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	e "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery/formatter"
	msg "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery/utils"
	kbd "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/keyboard"
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

func (h *Handler) Calls(c tele.Context) error {
	return c.Send(msg.Calls, tele.ModeMarkdown)
}

func (h *Handler) fetchSchedule(c tele.Context, groupID *int) (*pb.GroupScheduleResponse, string) {
	group, err := h.getGroupSchedule(c, groupID)
	if err != nil {
		switch {
		case errors.Is(err, e.ErrGroupNotFound):
			return nil, msg.GroupNotFound
		case errors.Is(err, e.ErrUserNoGroup):
			return nil, msg.UserNoGroup
		case errors.Is(err, e.ErrBadInput):
			return nil, msg.OnlyNumbers
		default:
			return nil, msg.Internal
		}
	}
	return group, ""
}

func (h *Handler) getGroupSchedule(c tele.Context, groupID *int) (*pb.GroupScheduleResponse, error) {
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
		group, err := h.uc.GetGroupScheduleByID(ctx, groupNum)
		if err != nil {
			slog.Warn("getGroupSchedule: failed by id", "chat_id", c.Chat().ID, "group_id", groupNum, "err", err)
			return nil, err
		}
		return group, nil
	}

	group, err := h.uc.GetGroupScheduleByChatID(ctx, c.Chat().ID)
	if err != nil {
		slog.Warn("getGroupSchedule:", "chat_id", c.Chat().ID, "err", err)
		return nil, err
	}

	return group, nil
}

func (h *Handler) handleEndTime(c tele.Context, schedule *pb.GroupScheduleResponse) error {
	dayIdx := utils.Day()
	day := schedule.Day[dayIdx]

	lastSubject := utils.FindLastSubject(day.Subjects)
	if lastSubject == -1 { // if no pairs in day
		return c.Send(formatter.FormatScheduleDay(schedule.Day[utils.Day(1)]), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard, kbd.InlineScheduleKeyboard(int(schedule.GroupNum)))
	}

	now := time.Now()

	endTime, ok := utils.GetEndTime(dayIdx, lastSubject)
	if ok {
		if now.After(endTime) || now.Equal(endTime) {
			return c.Send(formatter.FormatScheduleDay(schedule.Day[utils.Day(1)]), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard, kbd.InlineScheduleKeyboard(int(schedule.GroupNum)))
		}
	}

	return c.Send(formatter.FormatScheduleDay(schedule.Day[dayIdx]), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard, kbd.InlineScheduleKeyboard(int(schedule.GroupNum)))
}
