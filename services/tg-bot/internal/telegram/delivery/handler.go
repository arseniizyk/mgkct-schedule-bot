package delivery

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	userRepo "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository/postgres"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/state"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/state/memory"
)

type Handler struct {
	uc telegram.UserUsecase
	sm state.Manager
}

func NewHandler(uc telegram.UserUsecase) *Handler {
	return &Handler{
		uc: uc,
		sm: memory.NewMemory(),
	}
}

func (h *Handler) Help(c tele.Context) error {
	return c.Send(helpMsg)
}

func (h *Handler) Start(c tele.Context) error {
	user := c.Sender()
	u := &models.User{
		ChatID:   c.Chat().ID,
		Username: user.Username,
	}

	ctx := context.TODO()
	if err := h.uc.SaveUser(ctx, u); err != nil {
		slog.Error("can't save user", "username", u.Username, "chat_id", u.ChatID, "err", err)
	}

	return c.Send(startMsg, tele.ModeMarkdown, tele.NoPreview)
}

func (h *Handler) SetGroup(c tele.Context) error {
	if len(c.Args()) == 0 {
		if err := h.sm.Set(c.Chat().ID, state.WaitingGroup); err != nil {
			slog.Error("setgroup: can't set state", "chat_id", c.Chat().ID, "err", err)
			return c.Send("Внутренняя ошибка, попробуйте позже или получите расписание через /group 00")
		}
		return c.Send("Введите группу или /cancel для отмены")
	}

	groupID, ok := inputNum(c)
	if !ok {
		slog.Warn("setgroup: bad arg", "input", c.Args()[0])
		return c.Send(`Ошибка ввода: используйте только числовой номер группы`)
	}

	ctx := context.TODO()
	if err := h.uc.SetUserGroup(ctx, c.Chat().ID, groupID); err != nil {
		slog.Error("setgroup: failed to save group", "chat_id", c.Chat().ID, "group_id", groupID, "err", err)
		return c.Send(errInternal)
	}

	return c.Send(msgGroupSaved)
}

func (h *Handler) WaitingGroup(c tele.Context) error {
	group, err := strconv.Atoi(c.Text())
	if err != nil {
		slog.Error("can't parse group to int", "input", c.Text(), "err", err)
		return c.Send("Введите номер группы или /cancel для отмены.")
	}

	if err := h.uc.SetUserGroup(context.TODO(), c.Chat().ID, group); err != nil {
		slog.Error("waitingGroup: failed to set group", "chat_id", c.Chat().ID, "group_id", group, "err", err)
		return c.Send(errInternal)
	}

	if err := h.sm.Clear(c.Chat().ID); err != nil {
		slog.Warn("waiting group: can't clear state", "chat_id", c.Chat().ID, "err", err)
	}

	return c.Send(msgGroupSaved)
}

func (h *Handler) Group(c tele.Context) error {
	group, err := h.getGroupSchedule(c)
	if err != nil {
		return c.Send(errInternal)
	}
	return h.handleEndTime(c, group)
}

func (h *Handler) Week(c tele.Context) error {
	group, err := h.getGroupSchedule(c)
	if err != nil {
		return c.Send(errInternal)
	}
	return c.Send(formatScheduleWeek(group), tele.ModeMarkdown)
}

func (h *Handler) Day(c tele.Context) error {
	group, err := h.getGroupSchedule(c)
	if err != nil {
		return c.Send(err.Error())
	}
	return h.handleEndTime(c, group)
}

func (h *Handler) Calls(c tele.Context) error {
	return c.Send(callsMsg, tele.ModeMarkdown)
}

func (h *Handler) Cancel(c tele.Context) error {
	if err := h.sm.Clear(c.Chat().ID); err != nil {
		slog.Warn("cancel: can't clear state", "chat_id", c.Chat().ID, "err", err)
	}

	return c.Send(msgCancelled)
}

func (h *Handler) LogMessages(next tele.HandlerFunc) tele.HandlerFunc {
	return func(c tele.Context) error {
		slog.Info("incoming message",
			"chat_id", c.Chat().ID,
			"username", c.Sender().Username,
			"text", c.Text())
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

func (h *Handler) handleEndTime(c tele.Context, group *pb.GroupScheduleResponse) error {
	dayIdx := Day()
	day := group.Day[dayIdx]

	lastSubject := findLastSubject(day.Subjects)
	if lastSubject == -1 { // if no pairs in day
		return c.Send(formatScheduleDay(group.Day[Day(1)]), tele.ModeMarkdown)
	}

	now := time.Now()

	var endTime [2]int
	if dayIdx == 5 {
		endTime = weekendTimeEnd[lastSubject]
	} else {
		endTime = weekdaysTimeEnd[lastSubject]
	}

	if now.Hour() > endTime[0] || (now.Hour() == endTime[0] && now.Minute() >= weekdaysTimeEnd[lastSubject][1]) {
		return c.Send(formatScheduleDay(group.Day[Day(1)]), tele.ModeMarkdown)
	}

	return c.Send(formatScheduleDay(group.Day[dayIdx]), tele.ModeMarkdown)
}

func (h *Handler) getGroupSchedule(c tele.Context) (*pb.GroupScheduleResponse, error) {
	if groupID, ok := inputNum(c); ok {
		group, err := h.uc.GetGroupScheduleByID(context.Background(), groupID)
		if err != nil {
			slog.Error("getGroupSchedule: failed by id", "chat_id", c.Chat().ID, "group_id", groupID, "err", err)
			return nil, err
		}
		return group, nil
	}

	group, err := h.uc.GetGroupScheduleByChatID(context.Background(), c.Chat().ID)
	if err != nil {
		if errors.Is(err, userRepo.ErrUserNotFound) {
			return nil, err
		}
		slog.Error("getGroupSchedule: failed by chat_id", "chat_id", c.Chat().ID, "err", err)
		return nil, err
	}
	return group, nil
}
