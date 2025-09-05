package delivery

import (
	"context"
	"errors"
	"log/slog"
	"strconv"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/state"
	userRepo "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository/postgres"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/usecase"
	tele "gopkg.in/telebot.v4"
)

type Handler struct {
	uc usecase.UserUseCase
	sm state.Manager
}

func NewHandler(stateManager state.Manager, uc usecase.UserUseCase) *Handler {
	h := Handler{
		uc: uc,
		sm: stateManager,
	}

	return &h
}

func (h *Handler) Help(c tele.Context) error {
	msg := `
Список доступных команд:
/setgroup - для привязки группы к вашей телеге
/group 00 - расписание группы, использует вашу если она привязана
/week - расписание вашей группы на неделю
/day - расписание вашей группы на день
/calls - расписание звонков
`
	return c.Send(msg)
}

func (h *Handler) Start(c tele.Context) error {
	msg := `
Приветствую, это  бот для просмотра расписания колледжа МГКЦТ
Желающим зарепортить баг или законтрибьютить - [тык](https://github.com/arseniizyk/mgkct-schedule-bot)
Список доступных команд:
/setgroup - для привязки группы к вашей телеге
/group 00 - расписание группы, использует вашу если она привязана
/week - расписание вашей группы на неделю
/day - расписание вашей группы на день
/calls - расписание звонков
`
	user := c.Sender()

	u := &models.User{
		ChatID:   c.Chat().ID,
		UserName: user.Username,
	}

	ctx := context.TODO()
	if err := h.uc.SaveUser(ctx, u); err != nil {
		slog.Error("can't save user", "username", u.UserName, "chat_id", u.ChatID, "err", err)
	}

	return c.Send(msg, tele.ModeMarkdown)
}

func (h *Handler) WaitingGroup(c tele.Context) error {
	groupStr := c.Text()
	group, err := strconv.Atoi(groupStr)
	if err != nil {
		slog.Error("can't parse group to int", "input", c.Text(), "err", err)
		return c.Send("Не удалось сохранить группу: введите номер группы или /cancel для отмены.")
	}

	if err := h.uc.SetUserGroup(context.TODO(), c.Chat().ID, group); err != nil {
		slog.Error("can't set user group", "group", group, "chat_id", c.Chat().ID, "err", err)
		return c.Send("Не удалось сохранить группу, попробуйте позже.")
	}

	if err := h.sm.Clear(c.Chat().ID); err != nil {
		slog.Warn("waiting group: can't clear state", "chat_id", c.Chat().ID, "err", err)
	}

	return c.Send("Группа успешно установлена!")
}

func (h *Handler) SetGroup(c tele.Context) error {
	if len(c.Args()) == 0 {
		if err := h.sm.Set(c.Chat().ID, state.WaitingGroup); err != nil {
			slog.Error("setgroup: can't set state", "chat_id", c.Chat().ID, "err", err)
			return c.Send("Внутренняя ошибка, попробуйте позже или получите расписание через /group 00")
		}
		return c.Send("Введите группу или /cancel для отмены")
	}

	groupID, err := strconv.Atoi(c.Args()[0])
	if err != nil {
		slog.Warn("setgroup: bad arg", "input", c.Args()[0], "err", err)
		return c.Send(`Ошибка ввода группы, используйте только числовой номер группы`)
	}

	ctx := context.TODO()
	err = h.uc.SetUserGroup(ctx, c.Chat().ID, groupID)
	if err != nil {
		slog.Error("setgroup: error saving group", "chat_id", c.Chat().ID, "group_id", groupID, "err", err)
		return c.Send("Ошибка сохранения группы, попробуйте позже")
	}

	return c.Send("Группа успешно сохранена")
}

func (h *Handler) Group(c tele.Context) error {
	if len(c.Args()) == 0 {
		group, err := h.uc.GetGroupScheduleByChatID(context.TODO(), c.Chat().ID)
		if err == nil { // IF ERR == NIL, NO ERROR
			msg := formatScheduleDay(group)
			return c.Send(msg)
		}

		if errors.Is(err, userRepo.ErrUserNotFound) {
			slog.Warn("user's group not found", "chat_id", c.Chat().ID)
			return c.Send("Введите /group с номером группы, пример: /group 00")
		}

		slog.Error("error getting group schedule by chat_id", "chat_id", c.Chat().ID, "err", err)
		return c.Send("Ошибка получения расписания группы, попробуйте позже.")
	}

	groupID, err := strconv.Atoi(c.Args()[0])
	if err != nil {
		return c.Send("Некорректная группа")
	}

	group, err := h.uc.GetGroupScheduleByID(context.TODO(), groupID)
	if err != nil {
		slog.Error("error getting group schedule by ID", "group_id", groupID, "err", err)
		return c.Send("Ошибка получения расписания группы")
	}

	msg := formatScheduleDay(group)
	return c.Send(msg)
}

func (h *Handler) Week(c tele.Context) error {
	group, err := h.uc.GetGroupScheduleByChatID(context.TODO(), c.Chat().ID)
	if err != nil && !errors.Is(err, userRepo.ErrUserNotFound) {
		slog.Error("error getting user's group by chat_id", "chat_id", c.Chat().ID, "err", err)
		return c.Send("Ошибка при получении группы, попробуйте позже")
	}

	if errors.Is(err, userRepo.ErrUserNotFound) {
		slog.Warn("user's group not found", "chat_id", c.Chat().ID, "err", err)
		return c.Send("Вам необходимо установить группу через /setgroup или использовать /group 00")
	}

	msg := formatScheduleWeek(group)
	return c.Send(msg)
}

func (h *Handler) Day(c tele.Context) error {
	group, err := h.uc.GetGroupScheduleByChatID(context.TODO(), c.Chat().ID)
	if err != nil && !errors.Is(err, userRepo.ErrUserNotFound) {
		slog.Error("error getting user's group by chat_id", "chat_id", c.Chat().ID, "err", err)
		return c.Send("Ошибка при получении группы, попробуйте позже")
	}

	if errors.Is(err, userRepo.ErrUserNotFound) {
		slog.Warn("user's group not found", "chat_id", c.Chat().ID)
		return c.Send("Вам необходимо установить группу через /setgroup или использовать /group 00")
	}

	msg := formatScheduleDay(group)
	return c.Send(msg)
}

func (h *Handler) Calls(c tele.Context) error {
	msg := `
*Будние дни:*
1: 9:00 - 9:45 | 9:55 - 10:40
2: 10:50 - 11:35 | 11:55 - 12:40
3: 13:00 - 13:45 | 13:55 - 14:40
4: 14:50 - 15:35 | 15:45 - 16:30
5: 16:40 - 17:25 | 17:35 - 18:20
6: 18:30 - 19:15 | 19:25 - 20:10

*Суббота:*
1: 9:00 - 9:45 | 9:55 - 10:40
2: 10:50 - 11:35 | 11:55 - 12:40
3: 12:50 - 13:35 | 13:45 - 14:30
4: 14:40 - 15:25 | 15:35 - 16:20
5: 16:30 - 17:15 | 17:25 - 18:10
6: 18:20 - 19:05 | 19:15 - 20:00
`

	return c.Send(msg, tele.ModeMarkdown)
}

func (h *Handler) Cancel(c tele.Context) error {
	if err := h.sm.Clear(c.Chat().ID); err != nil {
		slog.Warn("cancel: can't clear state", "chat_id", c.Chat().ID, "err", err)
	}

	return c.Send("Действие отменено")
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
