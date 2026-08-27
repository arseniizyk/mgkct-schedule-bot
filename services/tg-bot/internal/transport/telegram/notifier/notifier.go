package notifier

import (
	"context"
	"fmt"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/messages"
)

type UserGetter interface {
	AllUsers(ctx context.Context) ([]entities.User, error)
	UserIDsByGroupID(ctx context.Context, groupID int) ([]int64, error)
	UserIDsByTeacherName(ctx context.Context, teacherName string) ([]int64, error)
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
		"group_id", resp.GetGroup().GetId(),
	)

	usersIDs, err := n.userGetter.UserIDsByGroupID(ctx, int(resp.GetGroup().GetId()))
	if err != nil {
		logger.ErrorContext(ctx, "failed to get users for group", "error", err)
		return err
	}

	for _, userID := range usersIDs {
		if err := n.sendUpdatedSchedule(userID, resp.GetGroup()); err != nil {
			logger.ErrorContext(ctx, "failed to send update to user", "user_id", userID, "error", err)
			continue
		}
		logger.InfoContext(ctx, "updated schedule sended", "chat_id", userID)
	}

	return nil
}

func (n *Notifier) HandleWeekUpdate(ctx context.Context) error {
	logger := n.log.With(
		"operation", "transport.telegram.notifier.HandleWeekUpdate",
	)

	users, err := n.userGetter.AllUsers(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "failed to get all users", "error", err)
		return err
	}

	for _, user := range users {
		err := n.sendUpdatedWeek(user)
		if err != nil {
			logger.ErrorContext(ctx, "failed to send week update to user", "user_id", user.ChatID, "error", err)
			continue
		}
		logger.InfoContext(ctx, "Updated week sended", "group_id", user.Group, "chat_id", user.ChatID)
	}

	return nil
}

func (n *Notifier) HandleTeacherScheduleUpdate(ctx context.Context, resp *pb.TeacherScheduleResponse) error {
	logger := n.log.With(
		"operation", "transport.telegram.notifier.HandleTeacherScheduleUpdate",
		"name", resp.GetTeacher().GetName(),
	)

	usersIDs, err := n.userGetter.UserIDsByTeacherName(ctx, resp.GetTeacher().GetName())
	if err != nil {
		logger.ErrorContext(ctx, "failed to get users for teacher", "error", err)
		return err
	}

	for _, userID := range usersIDs {
		if err := n.sendUpdatedTeacherSchedule(userID, resp.GetTeacher()); err != nil {
			logger.ErrorContext(ctx, "failed to send update to user", "user_id", userID, "error", err)
			continue
		}
		logger.InfoContext(ctx, "updated teacher schedule sended", "chat_id", userID)
	}

	return nil
}

func (n *Notifier) HandleTeacherWeekUpdate(ctx context.Context) error {
	logger := n.log.With(
		"operation", "transport.telegram.notifier.HandleTeacherWeekUpdate",
	)

	users, err := n.userGetter.AllUsers(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "failed to get all users", "error", err)
		return err
	}

	for _, user := range users {
		if user.TeacherName == "" {
			continue
		}

		err := n.sendUpdatedTeacherWeek(user)
		if err != nil {
			logger.ErrorContext(ctx, "failed to send teacher week update to user", "user_id", user.ChatID, "error", err)
			continue
		}
		logger.InfoContext(ctx, "Updated teacher week sended", "teacher_name", user.TeacherName, "chat_id", user.ChatID)
	}

	return nil
}

func (n *Notifier) sendUpdatedSchedule(chatID int64, group *pb.Group) error {
	body := formatter.FormatScheduleWeek(group)
	n.log.Debug("sending updated Schedule", "chat_id", chatID, "group_id", group.GetId())

	return n.sendLong(chatID, messages.ScheduleUpdatedTitle, body)
}

func (n *Notifier) sendLong(chatID int64, title, body string) error {
	msg := fmt.Sprintf("%s\n%s", title, body)
	for _, chunk := range formatter.SplitMessage(msg, 4096) {
		if _, err := n.bot.Send(tele.ChatID(chatID), chunk, tele.ModeMarkdown); err != nil {
			return err
		}
	}
	return nil
}

func (n *Notifier) sendUpdatedWeek(user entities.User) error {
	_, err := n.bot.Send(tele.ChatID(user.ChatID), messages.WeekUpdated, keyboard.InlineScheduleKeyboard(user.Group))

	return err
}

func (n *Notifier) sendUpdatedTeacherSchedule(chatID int64, teacher *pb.Teacher) error {
	body := formatter.FormatTeacherScheduleWeek(teacher)
	n.log.Debug("sending updated teacher schedule", "chat_id", chatID, "name", teacher.GetName())

	return n.sendLong(chatID, messages.ScheduleUpdatedTitle, body)
}

func (n *Notifier) sendUpdatedTeacherWeek(user entities.User) error {
	_, err := n.bot.Send(tele.ChatID(user.ChatID), messages.TeacherWeekUpdated, keyboard.InlineTeacherScheduleKeyboard(user.ChatID))

	return err
}
