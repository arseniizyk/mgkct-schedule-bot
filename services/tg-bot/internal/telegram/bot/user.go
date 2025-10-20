package bot

import (
	"context"
	"log/slog"
	"time"

	kbd "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/bot/keyboard"
	msg "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/bot/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/state"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
)

func (h *Handler) Help(c tele.Context) error {
	return c.Send(msg.Help)
}

func (h *Handler) Start(c tele.Context) error {
	u := models.User{
		ChatID:   c.Chat().ID,
		Username: c.Sender().Username,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.userRepo.Save(ctx, u); err != nil {
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

	if err := h.userRepo.SetGroup(ctx, c.Chat().ID, groupID); err != nil {
		slog.Error("setgroup: failed to save group", "chat_id", c.Chat().ID, "group_id", groupID, "err", err)
		return c.Send(msg.InternalTryWith)
	}

	return c.Send(msg.GroupSaved, kbd.ReplyScheduleKeyboard)
}
