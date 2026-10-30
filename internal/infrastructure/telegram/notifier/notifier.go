package notifier

import (
	"context"
	"errors"
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

type BotSender interface {
	Send(to tele.Recipient, what any, opts ...any) (*tele.Message, error)
}

type Notifier struct {
	log       *slog.Logger
	bot       BotSender
	userStore UserStore
	adminIDs  []int64
	throttle  <-chan time.Time
}

func New(log *slog.Logger, bot BotSender, userStore UserStore, adminIDs []int64) *Notifier {
	// Лимит Telegram: около 30 сообщений в секунду.
	// Используем тикер на 40ms (25 сообщений в секунду) для гарантированной защиты от 429 Too Many Requests.
	ticker := time.NewTicker(40 * time.Millisecond)

	return &Notifier{
		log:       log,
		bot:       bot,
		userStore: userStore,
		adminIDs:  adminIDs,
		throttle:  ticker.C,
	}
}

// UsersForNotification возвращает список пользователей, подписанных на уведомления о расписании на завтра.
func (n *Notifier) UsersForNotification(ctx context.Context) ([]entities.User, error) {
	return n.userStore.UsersForEveningNotification(ctx)
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
				_, _ = n.bot.Send(tele.ChatID(u.ChatID), messages.WeekUpdated, keyboard.InlineScheduleKeyboard(u.Group, week))
			case u.TeacherName != "":
				_, _ = n.bot.Send(tele.ChatID(u.ChatID), messages.TeacherWeekUpdated, keyboard.InlineTeacherScheduleKeyboard(u.TeacherName, week))
			default:
				_, _ = n.bot.Send(tele.ChatID(u.ChatID), messages.WeekUpdated)
			}
		}
	}

	return nil
}

// HandleGroupScheduleUpdate рассылает обновленное расписание студентам конкретной группы.
func (n *Notifier) HandleGroupScheduleUpdate(ctx context.Context, group *entities.Group, diff *entities.ScheduleDiff) error {
	log := n.log.With("operation", "notifier.Notifier.HandleGroupScheduleUpdate", "group_id", group.ID)

	userIDs, err := n.userStore.UserIDsByGroupID(ctx, int(group.ID))
	if err != nil {
		log.ErrorContext(ctx, "failed getting users for group update", "error", err)
		return err
	}

	if len(userIDs) == 0 {
		return nil
	}

	var msgText string
	var parseMode string
	if diff != nil {
		msgText = formatter.FormatGroupScheduleDiff(diff)
		parseMode = tele.ModeHTML
	} else {
		body := formatter.FormatScheduleWeek(group)
		msgText = fmt.Sprintf("%s\n%s", messages.ScheduleUpdatedTitle, body)
		parseMode = tele.ModeMarkdown
	}

	if msgText == "" {
		return nil
	}

	chunks := formatter.SplitMessage(msgText, 4096)
	kbd := keyboard.InlineScheduleKeyboard(int(group.ID), group.Week)

	for _, chatID := range userIDs {
		for i, chunk := range chunks {
			isLast := i == len(chunks)-1
			var sendOpts []any
			sendOpts = append(sendOpts, parseMode)
			if isLast && kbd != nil {
				sendOpts = append(sendOpts, kbd)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-n.throttle:
				_, err := n.bot.Send(tele.ChatID(chatID), chunk, sendOpts...)
				if err != nil {
					log.WarnContext(ctx, "failed sending group update to user", "chat_id", chatID, "error", err)
				}
			}
		}
	}

	return nil
}

// HandleTeacherScheduleUpdate рассылает обновленное расписание подписчикам преподавателя.
func (n *Notifier) HandleTeacherScheduleUpdate(ctx context.Context, teacher *entities.Teacher, diff *entities.ScheduleDiff) error {
	log := n.log.With("operation", "notifier.Notifier.HandleTeacherScheduleUpdate", "name", teacher.Name)

	userIDs, err := n.userStore.UserIDsByTeacherName(ctx, teacher.Name)
	if err != nil {
		log.ErrorContext(ctx, "failed getting users for teacher update", "error", err)
		return err
	}

	if len(userIDs) == 0 {
		return nil
	}

	var msgText string
	var parseMode string
	if diff != nil {
		msgText = formatter.FormatTeacherScheduleDiff(diff)
		parseMode = tele.ModeHTML
	} else {
		body := formatter.FormatTeacherScheduleWeek(teacher)
		msgText = fmt.Sprintf("%s\n%s", messages.ScheduleUpdatedTitle, body)
		parseMode = tele.ModeMarkdown
	}

	if msgText == "" {
		return nil
	}

	chunks := formatter.SplitMessage(msgText, 4096)
	kbd := keyboard.InlineTeacherScheduleKeyboard(teacher.Name, teacher.Week)

	for _, chatID := range userIDs {
		for i, chunk := range chunks {
			isLast := i == len(chunks)-1
			var sendOpts []any
			sendOpts = append(sendOpts, parseMode)
			if isLast && kbd != nil {
				sendOpts = append(sendOpts, kbd)
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-n.throttle:
				_, err := n.bot.Send(tele.ChatID(chatID), chunk, sendOpts...)
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
	if len(n.adminIDs) == 0 {
		return nil
	}
	errMsg := "неизвестная ошибка"
	if err != nil {
		errMsg = err.Error()
	}
	msg := fmt.Sprintf(
		"⚠️ *Внимание! Сбой парсера расписания!*\n\nСайт колледжа недоступен или парсер упал *%d* раз подряд.\n\n*Ошибка:* `%s`\n_Время: %s_",
		fails,
		errMsg,
		time.Now().Format("15:04:05 02.01.2006"),
	)
	var firstErr error
	for _, adminID := range n.adminIDs {
		if _, sendErr := n.bot.Send(tele.ChatID(adminID), msg, tele.ModeMarkdown); sendErr != nil && firstErr == nil {
			firstErr = sendErr
		}
	}
	return firstErr
}

// HandleScraperRecovered отправляет уведомление о восстановлении работы парсера.
func (n *Notifier) HandleScraperRecovered(ctx context.Context) error {
	if len(n.adminIDs) == 0 {
		return nil
	}
	msg := fmt.Sprintf(
		"✅ *Парсер восстановил работу!*\n\nСайт колледжа снова отвечает, расписание успешно обработано.\n_Время: %s_",
		time.Now().Format("15:04:05 02.01.2006"),
	)
	var firstErr error
	for _, adminID := range n.adminIDs {
		if _, sendErr := n.bot.Send(tele.ChatID(adminID), msg, tele.ModeMarkdown); sendErr != nil && firstErr == nil {
			firstErr = sendErr
		}
	}
	return firstErr
}

// CalculateTomorrowDate рассчитывает целевую дату следующего учебного дня и индекс дня (0..5 для Пн..Сб).
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
	case time.Friday:
		// В пятницу после окончания пар следующее расписание — на субботу (индекс 5)
		targetDate = localNow.AddDate(0, 0, 1)
		dayIdx = 5
	default:
		targetDate = localNow.AddDate(0, 0, 1)
		dayIdx = int(weekday) // Для Пн(1) -> Вт(индекс 1 в grp.Days), для Вт(2) -> Ср(индекс 2)
	}
	return targetDate, dayIdx
}

func isSameWeek(t1, t2 time.Time) bool {
	y1, w1 := t1.ISOWeek()
	y2, w2 := t2.ISOWeek()
	return y1 == y2 && w1 == w2
}

// FormatNextDayDigest формирует текст сообщения с расписанием на следующий учебный день.
func FormatNextDayDigest(
	ctx context.Context,
	user entities.User,
	targetDate time.Time,
	dayIdx int,
	schedProvider ScheduleProvider,
	teacherProvider TeacherProvider,
) (string, error) {
	targetMonday := targetDate.AddDate(0, 0, -dayIdx)

	minskLoc := time.FixedZone("MSK", 3*3600)
	localNow := time.Now().In(minskLoc)
	headerDay := "завтра"
	if localNow.Weekday() == time.Saturday {
		headerDay = "понедельник"
	}

	if user.Group > 0 {
		grp, err := schedProvider.GetGroupScheduleByWeek(ctx, int32(user.Group), targetMonday)
		if err != nil {
			grp, err = schedProvider.GetGroupLatestSchedule(ctx, int32(user.Group))
			if err == nil && !isSameWeek(grp.Week, targetMonday) {
				return "", fmt.Errorf("schedule for week %s not published yet", targetMonday.Format("02.01.2006"))
			}
		}
		if err != nil {
			return "", err
		}

		filtered := formatter.FilterGroupForSubgroup(grp, user.Subgroup)
		if dayIdx < 0 || dayIdx >= len(filtered.Days) || filtered.Days[dayIdx] == nil {
			return "", fmt.Errorf("dayIdx %d out of range", dayIdx)
		}

		dayMsg, err := formatter.FormatScheduleDayAt(filtered, dayIdx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("📚 *Расписание на %s (%s)*:\n\n%s", headerDay, targetDate.Format("02.01.2006"), dayMsg), nil
	}

	if user.TeacherName != "" {
		t, err := teacherProvider.GetTeacherScheduleByWeek(ctx, user.TeacherName, targetMonday)
		if err != nil {
			t, err = teacherProvider.GetTeacherLatestSchedule(ctx, user.TeacherName)
			if err == nil && !isSameWeek(t.Week, targetMonday) {
				return "", fmt.Errorf("teacher schedule for week %s not published yet", targetMonday.Format("02.01.2006"))
			}
		}
		if err != nil {
			return "", err
		}

		if dayIdx < 0 || dayIdx >= len(t.Days) || t.Days[dayIdx] == nil {
			return "", fmt.Errorf("dayIdx %d out of range", dayIdx)
		}

		teachMsg, err := formatter.FormatTeacherScheduleDayAt(t, dayIdx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("📚 *Расписание на %s (%s)*:\n\n%s", headerDay, targetDate.Format("02.01.2006"), teachMsg), nil
	}

	return "", errors.New("neither group nor teacher specified")
}

// SendNextDayDigestForGroupSubgroup отправляет расписание на следующий день подписчикам конкретной группы и подгруппы.
func (n *Notifier) SendNextDayDigestForGroupSubgroup(
	ctx context.Context,
	schedProvider ScheduleProvider,
	targetGroup int,
	targetSubgroup int,
) ([]int64, error) {
	log := n.log.With(
		"operation", "notifier.Notifier.SendNextDayDigestForGroupSubgroup",
		"group", targetGroup,
		"subgroup", targetSubgroup,
	)
	users, err := n.userStore.UsersForEveningNotification(ctx)
	if err != nil {
		log.ErrorContext(ctx, "failed getting users for digest", "error", err)
		return nil, err
	}

	targetDate, dayIdx := CalculateTomorrowDate(time.Now())

	dummyUser := entities.User{
		Group:    targetGroup,
		Subgroup: targetSubgroup,
	}

	msg, err := FormatNextDayDigest(ctx, dummyUser, targetDate, dayIdx, schedProvider, nil)
	if err != nil {
		log.WarnContext(ctx, "failed formatting next day digest", "error", err)
		return nil, err
	}

	var sentIDs []int64
	for _, u := range users {
		if u.Group == targetGroup && u.Subgroup == targetSubgroup {
			select {
			case <-ctx.Done():
				return sentIDs, ctx.Err()
			case <-n.throttle:
				if _, sendErr := n.bot.Send(tele.ChatID(u.ChatID), msg, tele.ModeMarkdown); sendErr != nil {
					log.WarnContext(ctx, "failed sending next day digest", "chat_id", u.ChatID, "error", sendErr)
				} else {
					sentIDs = append(sentIDs, u.ChatID)
				}
			}
		}
	}

	return sentIDs, nil
}

// SendNextDayDigestForTeacher отправляет расписание на следующий день подписчикам преподавателя.
func (n *Notifier) SendNextDayDigestForTeacher(
	ctx context.Context,
	teacherProvider TeacherProvider,
	targetTeacher string,
) ([]int64, error) {
	log := n.log.With("operation", "notifier.Notifier.SendNextDayDigestForTeacher", "teacher", targetTeacher)
	users, err := n.userStore.UsersForEveningNotification(ctx)
	if err != nil {
		log.ErrorContext(ctx, "failed getting users for digest", "error", err)
		return nil, err
	}

	targetDate, dayIdx := CalculateTomorrowDate(time.Now())
	dummyUser := entities.User{TeacherName: targetTeacher}

	msg, err := FormatNextDayDigest(ctx, dummyUser, targetDate, dayIdx, nil, teacherProvider)
	if err != nil {
		log.WarnContext(ctx, "failed formatting next day digest for teacher", "error", err)
		return nil, err
	}

	var sentIDs []int64
	for _, u := range users {
		if u.TeacherName == targetTeacher {
			select {
			case <-ctx.Done():
				return sentIDs, ctx.Err()
			case <-n.throttle:
				if _, sendErr := n.bot.Send(tele.ChatID(u.ChatID), msg, tele.ModeMarkdown); sendErr != nil {
					log.WarnContext(ctx, "failed sending next day digest", "chat_id", u.ChatID, "error", sendErr)
				} else {
					sentIDs = append(sentIDs, u.ChatID)
				}
			}
		}
	}

	return sentIDs, nil
}

// SendNextDayDigestForTarget отправляет расписание на следующий день всем подписчикам конкретной группы или преподавателя.
func (n *Notifier) SendNextDayDigestForTarget(
	ctx context.Context,
	schedProvider ScheduleProvider,
	teacherProvider TeacherProvider,
	targetGroup int,
	targetTeacher string,
) ([]int64, error) {
	if targetGroup > 0 {
		return n.SendNextDayDigestForGroupSubgroup(ctx, schedProvider, targetGroup, 0)
	}
	if targetTeacher != "" {
		return n.SendNextDayDigestForTeacher(ctx, teacherProvider, targetTeacher)
	}
	return nil, errors.New("neither group nor teacher specified")
}

// SendEveningDigest рассылает расписание на завтра всем пользователям с включенными уведомлениями.
// Опциональный параметр skipChatIDs позволяет пропустить пользователей, уже получивших дайджест ранее.
func (n *Notifier) SendEveningDigest(
	ctx context.Context,
	schedProvider ScheduleProvider,
	teacherProvider TeacherProvider,
	skipChatIDs ...map[int64]bool,
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

	var skip map[int64]bool
	if len(skipChatIDs) > 0 && skipChatIDs[0] != nil {
		skip = skipChatIDs[0]
	}

	targetDate, dayIdx := CalculateTomorrowDate(time.Now())

	sentCount := 0
	for _, u := range users {
		if skip != nil && skip[u.ChatID] {
			continue
		}

		select {
		case <-ctx.Done():
			log.WarnContext(ctx, "evening digest cancelled by context")
			return ctx.Err()
		case <-n.throttle:
			msg, err := FormatNextDayDigest(ctx, u, targetDate, dayIdx, schedProvider, teacherProvider)
			if err != nil {
				continue
			}
			if _, sendErr := n.bot.Send(tele.ChatID(u.ChatID), msg, tele.ModeMarkdown); sendErr == nil {
				sentCount++
				if skip != nil {
					skip[u.ChatID] = true
				}
			}
		}
	}

	log.InfoContext(ctx, "evening digest broadcast completed", "total_users", len(users), "sent_count", sentCount)
	return nil
}
