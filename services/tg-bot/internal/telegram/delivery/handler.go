package delivery

import (
	"log/slog"
	"strconv"
	"strings"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery/formatter"
	msg "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery/messages"
	kbd "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/keyboard"
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
