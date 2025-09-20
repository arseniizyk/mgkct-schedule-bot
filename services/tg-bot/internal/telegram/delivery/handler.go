package delivery

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	e "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/errors"
	msg "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery/messages"
	kbd "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/keyboard"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/state"
)

type Handler struct {
	uc telegram.UserUsecase
	sm state.Manager
}

func NewHandler(uc telegram.UserUsecase, sm state.Manager) *Handler {
	return &Handler{
		uc: uc,
		sm: sm,
	}
}

func (h *Handler) Help(c tele.Context) error {
	return c.Send(msg.Help)
}

func (h *Handler) Start(c tele.Context) error {
	user := c.Sender()
	u := &models.User{
		ChatID:   c.Chat().ID,
		Username: user.Username,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.uc.SaveUser(ctx, u); err != nil {
		slog.Error("can't save user from /start", "username", u.Username, "chat_id", u.ChatID, "err", err)
	}

	return c.Send(msg.Start, tele.ModeMarkdown, tele.NoPreview)
}

func (h *Handler) SetGroup(c tele.Context) error {
	if len(c.Args()) == 0 {
		if err := h.sm.Set(c.Chat().ID, state.WaitingGroup); err != nil {
			slog.Error("setgroup: can't set state", "chat_id", c.Chat().ID, "state", state.WaitingGroup, "err", err)
			return c.Send(msg.InternalTryWith)
		}
		return c.Send(msg.WaitingGroup)
	}

	groupID, err := inputNum(c)
	if err != nil {
		slog.Warn("setgroup: bad arg", "input", c.Args()[0], "chat_id", c.Chat().ID, "username", c.Chat().Username)
		return c.Send(msg.OnlyNumbers)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.uc.SetUserGroup(ctx, c.Chat().ID, groupID); err != nil {
		slog.Error("setgroup: failed to save group", "chat_id", c.Chat().ID, "group_id", groupID, "err", err)
		return c.Send(msg.InternalTryWith)
	}

	return c.Send(msg.GroupSaved, kbd.ReplyScheduleKeyboard)
}

func (h *Handler) WaitingGroup(c tele.Context) error {
	group, err := strconv.Atoi(c.Text())
	if err != nil {
		slog.Error("can't parse group to int", "input", c.Text(), "err", err)
		return c.Send(msg.WaitingGroup)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.uc.SetUserGroup(ctx, c.Chat().ID, group); err != nil {
		slog.Error("waitingGroup: failed to set group", "chat_id", c.Chat().ID, "group_id", group, "err", err)
		return c.Send(msg.InternalTryWith)
	}

	if err := h.sm.Clear(c.Chat().ID); err != nil {
		slog.Warn("waiting group: can't clear state", "chat_id", c.Chat().ID, "err", err)
	}

	return c.Send(msg.GroupSaved, kbd.ReplyScheduleKeyboard)
}

func (h *Handler) Week(c tele.Context) error {
	schedule, msg := h.fetchSchedule(c, nil)
	if msg != "" {
		return c.Send(msg)
	}
	return c.Send(formatScheduleWeek(schedule), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard)
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

func (h *Handler) Cancel(c tele.Context) error {
	if err := h.sm.Clear(c.Chat().ID); err != nil {
		slog.Warn("cancel: can't clear state", "chat_id", c.Chat().ID, "err", err)
	}

	return c.Send(msg.Cancelled)
}

func (h *Handler) LogMessages(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		if c.Callback() != nil {
			slog.Info("incoming callback",
				"chat_id", c.Chat().ID,
				"username", c.Sender().Username,
				"data", c.Callback().Data,
			)
		} else {
			slog.Info("incoming message",
				"chat_id", c.Chat().ID,
				"username", c.Sender().Username,
				"text", c.Text())
		}
		return next(c)
	}
}

func (h *Handler) HandleState(c tele.Context) error {
	userState, exists := h.sm.Get(c.Chat().ID)
	if !exists {
		return h.Help(c)
	}

	switch userState {
	case state.WaitingGroup:
		return h.WaitingGroup(c)
	default:
		return h.Cancel(c)
	}
}

func (h *Handler) HandleCallback(c tele.Context) error {
	callback := c.Callback()
	if err := c.Respond(); err != nil {
		slog.Warn("HandleCallback: respond failed", "err", err)
	}

	switch {
	case strings.Contains(callback.Data, kbd.Week):
		parts := strings.Split(callback.Data, "|")
		groupID, err := strconv.Atoi(parts[1])
		if err != nil {
			slog.Warn("callback: invalid data", "data", callback.Data, "chat_id", c.Chat().ID, "err", err)
			return c.Respond(&tele.CallbackResponse{Text: msg.Internal, ShowAlert: true})
		}

		schedule, msg := h.fetchSchedule(c, &groupID)
		if msg != "" {
			return c.Send(msg)
		}
		return c.Edit(formatScheduleWeek(schedule), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard, kbd.InlineEmptyKeyboard)

	default:
		slog.Warn("undefined callback", "chat_id", c.Chat().ID, "username", c.Sender().Username, "data", callback.Data)
		return c.Respond(&tele.CallbackResponse{Text: msg.Internal, ShowAlert: true})
	}
}

func (h *Handler) handleEndTime(c tele.Context, schedule *pb.GroupScheduleResponse) error {
	dayIdx := Day()
	day := schedule.Day[dayIdx]

	lastSubject := findLastSubject(day.Subjects)
	if lastSubject == -1 { // if no pairs in day
		return c.Send(formatScheduleDay(schedule.Day[Day(1)]), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard, kbd.InlineScheduleKeyboard(int(schedule.GroupNum)))
	}

	now := time.Now()

	var endTime [2]int
	if dayIdx == 5 {
		endTime = weekendTimeEnd[lastSubject]
	} else {
		endTime = weekdaysTimeEnd[lastSubject]
	}

	if now.Hour() > endTime[0] || (now.Hour() == endTime[0] && now.Minute() >= endTime[1]) {
		return c.Send(formatScheduleDay(schedule.Day[Day(1)]), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard, kbd.InlineScheduleKeyboard(int(schedule.GroupNum)))
	}

	return c.Send(formatScheduleDay(schedule.Day[dayIdx]), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard, kbd.InlineScheduleKeyboard(int(schedule.GroupNum)))
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
		groupNum, err = inputNum(c)
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
