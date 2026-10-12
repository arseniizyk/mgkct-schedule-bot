package notifier

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
)

type UserStore interface {
	AllUsers(ctx context.Context) ([]entities.User, error)
	UserIDsByGroupID(ctx context.Context, groupID int) ([]int64, error)
	UserIDsByTeacherName(ctx context.Context, teacherName string) ([]int64, error)
}

type Notifier struct {
	log       *slog.Logger
	bot       *tele.Bot
	userStore UserStore
	throttle  <-chan time.Time
}

func New(log *slog.Logger, bot *tele.Bot, userStore UserStore) *Notifier {
	// Лимит Telegram: около 30 сообщений в секунду.
	// Используем тикер на 40ms (25 сообщений в секунду) для гарантированной защиты от 429 Too Many Requests.
	ticker := time.NewTicker(40 * time.Millisecond)

	return &Notifier{
		log:       log,
		bot:       bot,
		userStore: userStore,
		throttle:  ticker.C,
	}
}

// HandleNewWeek рассылает уведомление о новой неделе ВСЕМ пользователям.
// Вызывается скрапером строго ОДИН РАЗ при обнаружении новой недели.
func (n *Notifier) HandleNewWeek(ctx context.Context, week time.Time) error {
	log := n.log.With("operation", "notifier.Notifier.HandleNewWeek", "week", week)
	log.InfoContext(ctx, "broadcasting new week notification")

	users, err := n.userStore.AllUsers(ctx)
	if err != nil {
		log.ErrorContext(ctx, "failed getting all users for new week broadcast", "error", err)
		return err
	}

	for _, u := range users {
		select {
		case <-ctx.Done():
			log.WarnContext(ctx, "broadcast cancelled by context")
			return ctx.Err()
		case <-n.throttle:
			if u.Group != 0 {
				_, _ = n.bot.Send(tele.ChatID(u.ChatID), messages.WeekUpdated, keyboard.InlineScheduleKeyboard(u.Group))
			} else if u.TeacherName != "" {
				_, _ = n.bot.Send(tele.ChatID(u.ChatID), messages.TeacherWeekUpdated, keyboard.InlineTeacherScheduleKeyboard(u.TeacherName))
			} else {
				_, _ = n.bot.Send(tele.ChatID(u.ChatID), messages.WeekUpdated)
			}
		}
	}

	return nil
}

// HandleGroupScheduleUpdate рассылает обновленное расписание студентам конкретной группы.
func (n *Notifier) HandleGroupScheduleUpdate(ctx context.Context, group *entities.Group) error {
	log := n.log.With("operation", "notifier.Notifier.HandleGroupScheduleUpdate", "group_id", group.ID)

	userIDs, err := n.userStore.UserIDsByGroupID(ctx, int(group.ID))
	if err != nil {
		log.ErrorContext(ctx, "failed getting users for group update", "error", err)
		return err
	}

	if len(userIDs) == 0 {
		return nil
	}

	body := formatter.FormatScheduleWeek(group)
	msgText := fmt.Sprintf("%s\n%s", messages.ScheduleUpdatedTitle, body)
	chunks := formatter.SplitMessage(msgText, 4096)

	for _, chatID := range userIDs {
		for _, chunk := range chunks {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-n.throttle:
				_, err := n.bot.Send(tele.ChatID(chatID), chunk, tele.ModeMarkdown)
				if err != nil {
					log.WarnContext(ctx, "failed sending group update to user", "chat_id", chatID, "error", err)
				}
			}
		}
	}

	return nil
}

// HandleTeacherScheduleUpdate рассылает обновленное расписание подписчикам преподавателя.
func (n *Notifier) HandleTeacherScheduleUpdate(ctx context.Context, teacher *entities.Teacher) error {
	log := n.log.With("operation", "notifier.Notifier.HandleTeacherScheduleUpdate", "name", teacher.Name)

	userIDs, err := n.userStore.UserIDsByTeacherName(ctx, teacher.Name)
	if err != nil {
		log.ErrorContext(ctx, "failed getting users for teacher update", "error", err)
		return err
	}

	if len(userIDs) == 0 {
		return nil
	}

	body := formatter.FormatTeacherScheduleWeek(teacher)
	msgText := fmt.Sprintf("%s\n%s", messages.ScheduleUpdatedTitle, body)
	chunks := formatter.SplitMessage(msgText, 4096)

	for _, chatID := range userIDs {
		for _, chunk := range chunks {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-n.throttle:
				_, err := n.bot.Send(tele.ChatID(chatID), chunk, tele.ModeMarkdown)
				if err != nil {
					log.WarnContext(ctx, "failed sending teacher update to user", "chat_id", chatID, "error", err)
				}
			}
		}
	}

	return nil
}
