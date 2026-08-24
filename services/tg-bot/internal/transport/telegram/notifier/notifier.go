package notifier

import (
	"context"
	"fmt"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
	tele "gopkg.in/telebot.v4"
)

type UserGetter interface {
	AllUsers(ctx context.Context) ([]entities.User, error)
	UserIDsByGroupID(ctx context.Context, groupID int) ([]int64, error)
}

type Notifier struct {
	bot        *tele.Bot
	log        *slog.Logger
	userGetter UserGetter
}

func New(log *slog.Logger, bot *tele.Bot, userGetter UserGetter) *Notifier {
	return &Notifier{
		bot:        bot,
		log:        log,
		userGetter: userGetter,
	}
}

func (n *Notifier) HandleScheduleUpdate(ctx context.Context, resp *pb.GroupScheduleResponse) error {
	logger := n.log.With(
		"operation", "transport.telegram.notifier.HandleScheduleUpdate",
		"group_id", resp.Group.GetId(),
	)

	usersIDs, err := n.userGetter.UserIDsByGroupID(ctx, int(resp.Group.GetId()))
	if err != nil {
		logger.Error("failed to get users for group", "error", err)
		return err
	}

	for _, userID := range usersIDs {
		if err := n.sendUpdatedSchedule(userID, resp.Group); err != nil {
			logger.Error("failed to send update to user", "user_id", userID, "error", err)
			continue
		}
		logger.Info("updated schedule sended", "chat_id", userID)
	}

	return nil
}

func (n *Notifier) HandleWeekUpdate(ctx context.Context) error {
	logger := n.log.With(
		"operation", "transport.telegram.notifier.HandleWeekUpdate",
	)

	users, err := n.userGetter.AllUsers(ctx)
	if err != nil {
		logger.Error("failed to get all users", "error", err)
		return err
	}

	for _, user := range users {
		err := n.sendUpdatedWeek(user)
		if err != nil {
			logger.Error("failed to send week update to user", "user_id", user.ChatID, "error", err)
			continue
		}
		logger.Info("Updated week sended", "group_id", user.Group, "chat_id", user.ChatID)
	}

	return nil
}

func (n *Notifier) sendUpdatedSchedule(chatID int64, group *pb.Group) error {
	msg := fmt.Sprintf("%s\n%s", messages.ScheduleUpdatedTitle, formatter.FormatScheduleWeek(group))
	n.log.Debug("sending updated Schedule", "chat_id", chatID, "group_id", group.Id)

	_, err := n.bot.Send(tele.ChatID(chatID), msg, tele.ModeMarkdown)
	return err
}

func (n *Notifier) sendUpdatedWeek(user entities.User) error {
	_, err := n.bot.Send(tele.ChatID(user.ChatID), messages.WeekUpdated, keyboard.InlineScheduleKeyboard(user.Group))

	return err
}
