package delivery

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	msg "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery/messages"
	kbd "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/keyboard"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery/utils"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/state"
)

func (h *Handler) SetGroup(c tele.Context) error {
	if len(c.Args()) == 0 {
		if err := h.sm.Set(c.Chat().ID, state.WaitingGroup); err != nil {
			slog.Error("setgroup: can't set state", "chat_id", c.Chat().ID, "state", state.WaitingGroup, "err", err)
			return c.Send(msg.InternalTryWith)
		}
		return c.Send(msg.WaitingGroup)
	}

	groupID, err := utils.InputNum(c)
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
