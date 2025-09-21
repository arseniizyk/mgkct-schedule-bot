package delivery

import (
	"context"
	"log/slog"
	"time"

	msg "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery/messages"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
)

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

func (h *Handler) Cancel(c tele.Context) error {
	if err := h.sm.Clear(c.Chat().ID); err != nil {
		slog.Warn("cancel: can't clear state", "chat_id", c.Chat().ID, "err", err)
	}

	return c.Send(msg.Cancelled)
}
