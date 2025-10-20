package bot

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	tele "gopkg.in/telebot.v4"

	telegramService "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/service"

	kbd "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/bot/keyboard"
	msg "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/bot/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/state"
)

type Handler struct {
	telegramService telegramService.Telegram
	userRepo        repository.User
	sm              state.Manager
	bot             *tele.Bot
}

func NewHandler(userRepo repository.User, service telegramService.Telegram, sm state.Manager, bot *tele.Bot) *Handler {
	return &Handler{
		telegramService: service,
		userRepo:        userRepo,
		sm:              sm,
		bot:             bot,
	}
}

func (h *Handler) WaitingGroup(c tele.Context) error {
	group, err := strconv.Atoi(c.Text())
	if err != nil {
		slog.Error("can't parse group to int", "input", c.Text(), "err", err)
		return c.Send(msg.WaitingGroup)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.userRepo.SetGroup(ctx, c.Chat().ID, group); err != nil {
		slog.Error("waitingGroup: failed to set group", "chat_id", c.Chat().ID, "group_id", group, "err", err)
		return c.Send(msg.InternalTryWith)
	}

	if err := h.sm.Clear(c.Chat().ID); err != nil {
		slog.Warn("waiting group: can't clear state", "chat_id", c.Chat().ID, "err", err)
	}

	return c.Send(msg.GroupSaved, kbd.ReplyScheduleKeyboard)
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
	case strings.Contains(callback.Data, kbd.CurrentWeek):
		return h.callbackCurrentWeek(c)

	case strings.Contains(callback.Data, kbd.PrevWeek):
		return h.callbackWeekNavigation(c)

	case strings.Contains(callback.Data, kbd.NextWeek):
		return h.callbackWeekNavigation(c)

	default:
		slog.Warn("undefined callback", "chat_id", c.Chat().ID, "username", c.Sender().Username, "data", callback.Data)
		return c.Respond(&tele.CallbackResponse{Text: msg.Internal, ShowAlert: true})
	}
}

func (h *Handler) HandleScheduleUpdate(ctx context.Context, g *pb.GroupScheduleResponse) error {
	users, err := h.userRepo.GetUsersByGroup(ctx, int(g.Group.Id))
	if err != nil {
		slog.Error("can't get users for group", "groupNum", g.Group.Id, "err", err)
		return err
	}

	for _, u := range users {
		err := h.SendUpdatedSchedule(u, g.Group)
		if err != nil {
			slog.Error("failed to send update to user", "userId", u, "err", err)
			continue
		}
		slog.Info("Updated schedule sended", "group_id", g.Group.Id, "chat_id", u)
	}

	return nil
}

func (h *Handler) HandleWeekUpdate(ctx context.Context) error {
	users, err := h.userRepo.SelectAll(ctx)
	if err != nil {
		slog.Error("can't select all users", "err", err)
		return err
	}

	for _, u := range users {
		err := h.SendUpdatedWeek(u)
		if err != nil {
			slog.Error("failed to send week update to user", "user_id", u.ChatID, "err", err)
			continue
		}
		slog.Info("Updated week sended", "group_id", u.Group, "chat_id", u.ChatID)
	}

	return nil
}
