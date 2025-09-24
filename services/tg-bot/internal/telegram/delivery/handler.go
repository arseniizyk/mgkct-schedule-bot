package delivery

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery/formatter"
	kbd "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery/keyboard"
	msg "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery/messages"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/state"
)

type Handler struct {
	uc  telegram.UserUsecase
	sm  state.Manager
	bot *tele.Bot
}

func NewHandler(uc telegram.UserUsecase, sm state.Manager, bot *tele.Bot) *Handler {
	return &Handler{
		uc:  uc,
		sm:  sm,
		bot: bot,
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

	if err := h.uc.SetUserGroup(ctx, c.Chat().ID, group); err != nil {
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
		return c.Edit(formatter.FormatScheduleWeek(schedule), tele.ModeMarkdown, kbd.ReplyScheduleKeyboard, kbd.InlineEmptyKeyboard)

	default:
		slog.Warn("undefined callback", "chat_id", c.Chat().ID, "username", c.Sender().Username, "data", callback.Data)
		return c.Respond(&tele.CallbackResponse{Text: msg.Internal, ShowAlert: true})
	}
}
