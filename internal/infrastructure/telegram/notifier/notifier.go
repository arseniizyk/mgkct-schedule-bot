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
	UsersForEveningNotification(ctx context.Context) ([]entities.User, error)
}

type ScheduleProvider interface {
	GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error)
	GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error)
}

type TeacherProvider interface {
	GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error)
	GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error)
	GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error)
}

type Notifier struct {
	log         *slog.Logger
	bot         *tele.Bot
	userStore   UserStore
	adminChatID int64
	throttle    <-chan time.Time
}

func New(log *slog.Logger, bot *tele.Bot, userStore UserStore, adminChatID int64) *Notifier {
	// Лимит Telegram: около 30 сообщений в секунду.
	// Используем тикер на 40ms (25 сообщений в секунду) для гарантированной защиты от 429 Too Many Requests.
	ticker := time.NewTicker(40 * time.Millisecond)

	return &Notifier{
		log:         log,
		bot:         bot,
		userStore:   userStore,
		adminChatID: adminChatID,
		throttle:    ticker.C,
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
			switch {
			case u.Group != 0:
				_, _ = n.bot.Send(tele.ChatID(u.ChatID), messages.WeekUpdated, keyboard.InlineScheduleKeyboard(u.Group))
			case u.TeacherName != "":
				_, _ = n.bot.Send(tele.ChatID(u.ChatID), messages.TeacherWeekUpdated, keyboard.InlineTeacherScheduleKeyboard(u.TeacherName))
			default:
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

// HandleScraperError отправляет алерт администратору о сбое парсера.
func (n *Notifier) HandleScraperError(ctx context.Context, err error, fails int) error {
	if n.adminChatID == 0 {
		return nil
	}
	msg := fmt.Sprintf(
		"⚠️ *Внимание! Сбой парсера расписания!*\n\nСайт колледжа недоступен или парсер упал *%d* раз подряд.\n\n*Ошибка:* `%s`\n_Время: %s_",
		fails,
		err.Error(),
		time.Now().Format("15:04:05 02.01.2006"),
	)
	_, sendErr := n.bot.Send(tele.ChatID(n.adminChatID), msg, tele.ModeMarkdown)
	return sendErr
}

// HandleScraperRecovered отправляет уведомление о восстановлении работы парсера.
func (n *Notifier) HandleScraperRecovered(ctx context.Context) error {
	if n.adminChatID == 0 {
		return nil
	}
	msg := fmt.Sprintf(
		"✅ *Парсер восстановил работу!*\n\nСайт колледжа снова отвечает, расписание успешно обработано.\n_Время: %s_",
		time.Now().Format("15:04:05 02.01.2006"),
	)
	_, sendErr := n.bot.Send(tele.ChatID(n.adminChatID), msg, tele.ModeMarkdown)
	return sendErr
}

// CalculateTomorrowDate рассчитывает дату и индекс дня (0..5 для Пн..Сб) для вечернего дайджеста.
func CalculateTomorrowDate(now time.Time) (targetDate time.Time, dayIdx int) {
	minskLoc := time.FixedZone("MSK", 3*3600)
	localNow := now.In(minskLoc)

	weekday := localNow.Weekday() // 0=Вс, 1=Пн, ... 6=Сб

	switch weekday {
	case time.Saturday:
		targetDate = localNow.AddDate(0, 0, 2)
		dayIdx = 0
	case time.Sunday:
		targetDate = localNow.AddDate(0, 0, 1)
		dayIdx = 0
	default:
		targetDate = localNow.AddDate(0, 0, 1)
		dayIdx = int(weekday) // для Пн(1) -> завтра Вт, индекс в массиве Days равен 1.
	}
	return targetDate, dayIdx
}

// SendEveningDigest рассылает расписание на завтра пользователям с включенными вечерними уведомлениями.
func (n *Notifier) SendEveningDigest(
	ctx context.Context,
	schedProvider ScheduleProvider,
	teacherProvider TeacherProvider,
) error {
	log := n.log.With("operation", "notifier.Notifier.SendEveningDigest")
	log.InfoContext(ctx, "starting evening digest broadcast")

	users, err := n.userStore.UsersForEveningNotification(ctx)
	if err != nil {
		log.ErrorContext(ctx, "failed getting users for evening digest", "error", err)
		return err
	}

	if len(users) == 0 {
		log.InfoContext(ctx, "no users subscribed to evening digest")
		return nil
	}

	targetDate, dayIdx := CalculateTomorrowDate(time.Now())
	// Понедельник недели для targetDate
	targetMonday := targetDate.AddDate(0, 0, -dayIdx)

	for _, u := range users {
		select {
		case <-ctx.Done():
			log.WarnContext(ctx, "evening digest cancelled by context")
			return ctx.Err()
		case <-n.throttle:
			if u.Group > 0 {
				grp, err := schedProvider.GetGroupScheduleByWeek(ctx, int32(u.Group), targetMonday)
				if err != nil {
					grp, err = schedProvider.GetGroupLatestSchedule(ctx, int32(u.Group))
				}
				if err != nil {
					continue
				}

				if dayIdx >= 0 && dayIdx < len(grp.Days) {
					dayMsg, err := formatter.FormatScheduleDayAt(grp, dayIdx)
					if err == nil {
						msg := fmt.Sprintf("🌙 *Расписание на завтра (%s)*:\n\n%s", targetDate.Format("02.01.2006"), dayMsg)
						_, _ = n.bot.Send(tele.ChatID(u.ChatID), msg, tele.ModeMarkdown)
					}
				}
			} else if u.TeacherName != "" {
				t, err := teacherProvider.GetTeacherScheduleByWeek(ctx, u.TeacherName, targetMonday)
				if err != nil {
					t, err = teacherProvider.GetTeacherLatestSchedule(ctx, u.TeacherName)
				}
				if err != nil {
					continue
				}

				if dayIdx >= 0 && dayIdx < len(t.Days) {
					teachMsg, err := formatter.FormatTeacherScheduleDayAt(t, dayIdx)
					if err == nil {
						member, _ := teacherProvider.GetFacultyMember(ctx, u.TeacherName)
						profile := formatter.FormatTeacherProfile(member)
						msg := fmt.Sprintf("🌙 *Расписание на завтра (%s)*:\n\n%s%s", targetDate.Format("02.01.2006"), profile, teachMsg)
						_, _ = n.bot.Send(tele.ChatID(u.ChatID), msg, tele.ModeMarkdown)
					}
				}
			}
		}
	}

	log.InfoContext(ctx, "evening digest broadcast completed", "total_users", len(users))
	return nil
}
