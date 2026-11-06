package admin

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/botutil"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/pkg/timeutil"
)

// BotSender — минимальный интерфейс для отправки сообщений от имени бота (используется при рассылке).
type BotSender interface {
	Send(to tele.Recipient, what any, opts ...any) (*tele.Message, error)
}

// UserRepo — интерфейс доступа к данным пользователей, необходимый для админских команд.
type UserRepo interface {
	AllUsersWithUsernames(ctx context.Context) ([]entities.User, error)
	AllUsers(ctx context.Context) ([]entities.User, error)
	GetActivityStats(ctx context.Context) (entities.ActivityStats, error)
	SetNotifyEvening(ctx context.Context, chatID int64, enabled bool) error
}

// StateManager — интерфейс управления состояниями пользователей (FSM).
type StateManager interface {
	Set(chatID int64, st state.State) error
	Get(chatID int64) (state.State, bool)
	Clear(chatID int64) error
}

// Limiter — интерфейс ограничения скорости рассылки.
type Limiter interface {
	Wait(ctx context.Context) error
}

// UserStats обрабатывает команду /userstats — статистика пользователей по группам/подгруппам.
func UserStats(log *slog.Logger, userRepo UserRepo) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		users, err := userRepo.AllUsersWithUsernames(ctx)
		if err != nil {
			log.Error("failed getting users with usernames", "error", err)
			return c.Send("❌ Ошибка получения списка пользователей.")
		}

		msg := formatUserStats(users)
		chunks := formatter.SplitMessage(msg, 4096)
		for _, chunk := range chunks {
			if err := c.Send(chunk); err != nil {
				log.Warn("failed sending userstats chunk", "error", err)
			}
		}
		return nil
	}
}

// BotInfo обрабатывает команду /botinfo — общая сводка о состоянии бота.
func BotInfo(log *slog.Logger, userRepo UserRepo) tele.HandlerFunc {
	return func(c tele.Context) error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		users, err := userRepo.AllUsersWithUsernames(ctx)
		if err != nil {
			log.Error("failed getting users for botinfo", "error", err)
			return c.Send("❌ Ошибка получения данных.")
		}

		total := len(users)
		var withGroup, withTeacher, noGroup, digestActive, digestDisabled int
		for _, u := range users {
			hasTarget := false
			switch {
			case u.Group > 0:
				withGroup++
				hasTarget = true
			case u.TeacherName != "":
				withTeacher++
				hasTarget = true
			default:
				noGroup++
			}

			if hasTarget {
				if u.NotifyEvening {
					digestActive++
				} else {
					digestDisabled++
				}
			}
		}

		actStats, err := userRepo.GetActivityStats(ctx)
		if err != nil {
			log.Warn("failed getting activity stats for botinfo", "error", err)
		}

		now := timeutil.Now()

		msg := fmt.Sprintf(
			"ℹ️ <b>Информация о боте</b>\n\n"+
				"👥 <b>Всего пользователей:</b> %d\n"+
				"  • Студентов (с группой): %d\n"+
				"  • Преподавателей: %d\n"+
				"  • Без группы: %d\n\n"+
				"📬 <b>Рассылка расписания:</b>\n"+
				"  • Получают вечерний дайджест: %d\n"+
				"  • Отключили / заблокировали: %d\n\n"+
				"📱 <b>Интерактивная активность (клики/команды):</b>\n"+
				"  • За 24 часа (DAU): %d\n"+
				"  • За 7 дней (WAU): %d\n"+
				"  • За 30 дней (MAU): %d\n\n"+
				"⏰ <b>Время сервера:</b> %s MSK",
			total, withGroup, withTeacher, noGroup,
			digestActive, digestDisabled,
			actStats.DAU, actStats.WAU, actStats.MAU,
			now.Format("15:04, Mon 02.01.2006"),
		)
		return c.Send(msg, tele.ModeHTML)
	}
}

// Broadcast обрабатывает команду /broadcast — начинает интерактивный режим рассылки.
func Broadcast(log *slog.Logger, sm StateManager) tele.HandlerFunc {
	return func(c tele.Context) error {
		if err := sm.Set(c.Chat().ID, state.WaitingBroadcast); err != nil {
			log.Error("failed setting broadcast state", "error", err)
			return c.Send("❌ Внутренняя ошибка.")
		}
		return c.Send("📢 Введите сообщение для рассылки всем пользователям или /cancel для отмены.")
	}
}

var (
	broadcastDraftsMu  sync.RWMutex
	broadcastDrafts    = make(map[int64]string)
	broadcastContextMu sync.RWMutex
	broadcastContext   context.Context
)

//nolint:fatcontext // broadcastContext stores global bot lifecycle context for async broadcasts
func SetBroadcastContext(ctx context.Context) {
	broadcastContextMu.Lock()
	defer broadcastContextMu.Unlock()
	broadcastContext = ctx
}

func getBroadcastContext() context.Context {
	broadcastContextMu.RLock()
	defer broadcastContextMu.RUnlock()
	if broadcastContext != nil {
		return broadcastContext
	}
	return context.Background()
}

func SetBroadcastDraft(chatID int64, text string) {
	broadcastDraftsMu.Lock()
	defer broadcastDraftsMu.Unlock()
	broadcastDrafts[chatID] = text
}

func GetBroadcastDraft(chatID int64) (string, bool) {
	broadcastDraftsMu.RLock()
	defer broadcastDraftsMu.RUnlock()
	t, ok := broadcastDrafts[chatID]
	return t, ok
}

func ClearBroadcastDraft(chatID int64) {
	broadcastDraftsMu.Lock()
	defer broadcastDraftsMu.Unlock()
	delete(broadcastDrafts, chatID)
}

func safeEdit(c tele.Context, what any, opt ...any) error {
	if c.Callback() != nil {
		return c.Edit(what, opt...)
	}
	return c.Send(what, opt...)
}

// HandleWaitingBroadcast вызывается при получении текста в состоянии WaitingBroadcast.
func HandleWaitingBroadcast(
	_ context.Context,
	_ *slog.Logger,
	c tele.Context,
	sm StateManager,
) error {
	chatID := c.Chat().ID
	text := strings.TrimSpace(c.Text())
	if text == "" {
		return c.Send("❌ Сообщение не может быть пустым.")
	}
	if len([]rune(text)) > 4000 {
		return c.Send("❌ Сообщение слишком длинное (максимум 4000 символов).")
	}

	_ = sm.Clear(chatID)
	SetBroadcastDraft(chatID, text)

	previewText := fmt.Sprintf("📢 Предпросмотр сообщения для рассылки:\n\n%s", text)
	return c.Send(previewText, keyboard.InlineBroadcastKeyboard())
}

// HandleBroadcastCallback обрабатывает подтверждение или отмену рассылки администратором.
func HandleBroadcastCallback(
	ctx context.Context,
	log *slog.Logger,
	c tele.Context,
	data string,
	userRepo UserRepo,
	bot BotSender,
	sm StateManager,
	adminIDs []int64,
) error {
	if !IsAdmin(adminIDs, c.Chat().ID) {
		return c.Respond(&tele.CallbackResponse{Text: "Доступ запрещен", ShowAlert: true})
	}

	var err error
	if strings.Contains(data, keyboard.BroadcastConfirm) {
		err = handleBroadcastConfirm(ctx, log, c, userRepo, bot)
	} else {
		err = handleBroadcastCancel(ctx, log, c, sm)
	}

	if err != nil {
		return err
	}
	return c.Respond()
}

func handleBroadcastConfirm(
	_ context.Context,
	log *slog.Logger,
	c tele.Context,
	userRepo UserRepo,
	bot BotSender,
) error {
	chatID := c.Chat().ID
	text, ok := GetBroadcastDraft(chatID)
	if !ok || strings.TrimSpace(text) == "" {
		return safeEdit(c, "❌ Сообщение для рассылки не найдено или устарело.")
	}
	ClearBroadcastDraft(chatID)

	if err := safeEdit(c, "🚀 Рассылка запущена..."); err != nil {
		log.Warn("failed editing broadcast status message", "error", err)
	}

	go executeBroadcast(getBroadcastContext(), log, chatID, text, userRepo, bot)

	return nil
}

func handleBroadcastCancel(
	_ context.Context,
	_ *slog.Logger,
	c tele.Context,
	sm StateManager,
) error {
	chatID := c.Chat().ID
	ClearBroadcastDraft(chatID)
	if sm != nil {
		_ = sm.Clear(chatID)
	}

	return safeEdit(c, "❌ Рассылка отменена.")
}

var (
	broadcastLimiterMu sync.RWMutex
	broadcastLimiter   Limiter
)

func SetBroadcastLimiter(l Limiter) {
	broadcastLimiterMu.Lock()
	defer broadcastLimiterMu.Unlock()
	broadcastLimiter = l
}

func getBroadcastLimiter() Limiter {
	broadcastLimiterMu.RLock()
	defer broadcastLimiterMu.RUnlock()
	return broadcastLimiter
}

func isForbiddenErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, tele.ErrBlockedByUser) || errors.Is(err, tele.ErrUserIsDeactivated) {
		return true
	}
	var teleErr *tele.Error
	return errors.As(err, &teleErr) && teleErr.Code == 403
}

func executeBroadcast(
	ctx context.Context,
	log *slog.Logger,
	adminChatID int64,
	text string,
	userRepo UserRepo,
	bot BotSender,
) {
	if bot == nil || userRepo == nil {
		log.Error("executeBroadcast: bot or userRepo is nil")
		return
	}

	users, err := userRepo.AllUsers(ctx)
	if err != nil {
		log.Error("broadcast: failed getting all users", "error", err)
		_, _ = bot.Send(tele.ChatID(adminChatID), "❌ Ошибка получения списка пользователей для рассылки.")
		return
	}

	if len(users) == 0 {
		_, _ = bot.Send(tele.ChatID(adminChatID), "Нет пользователей для рассылки.")
		return
	}

	limiter := getBroadcastLimiter()
	var sent, failed int
	for _, u := range users {
		if limiter != nil {
			if err := limiter.Wait(ctx); err != nil {
				log.Warn("broadcast cancelled by context")
				_, _ = bot.Send(tele.ChatID(adminChatID), fmt.Sprintf("⚠️ Рассылка прервана. Отправлено: %d / Ошибок: %d", sent, failed))
				return
			}
		} else {
			select {
			case <-ctx.Done():
				log.Warn("broadcast cancelled by context")
				_, _ = bot.Send(tele.ChatID(adminChatID), fmt.Sprintf("⚠️ Рассылка прервана. Отправлено: %d / Ошибок: %d", sent, failed))
				return
			case <-time.After(40 * time.Millisecond):
			}
		}

		if _, sendErr := botutil.SendWithFloodRetry(ctx, log, func() (*tele.Message, error) {
			return bot.Send(tele.ChatID(u.ChatID), text)
		}); sendErr != nil {
			log.Warn("broadcast: failed sending to user", "chat_id", u.ChatID, "error", sendErr)
			if isForbiddenErr(sendErr) {
				_ = userRepo.SetNotifyEvening(ctx, u.ChatID, false)
			}
			failed++
		} else {
			sent++
		}
	}

	_, _ = bot.Send(tele.ChatID(adminChatID), fmt.Sprintf("✅ Рассылка завершена.\nОтправлено: %d\nОшибок: %d", sent, failed))
}

func formatUserStats(users []entities.User) string {
	type groupData struct {
		bySubgroup map[int][]entities.User
	}

	groups := make(map[int]*groupData)
	var teachers []entities.User
	var noGroup []entities.User

	for _, u := range users {
		switch {
		case u.Group > 0:
			if groups[u.Group] == nil {
				groups[u.Group] = &groupData{bySubgroup: make(map[int][]entities.User)}
			}
			groups[u.Group].bySubgroup[u.Subgroup] = append(groups[u.Group].bySubgroup[u.Subgroup], u)
		case u.TeacherName != "":
			teachers = append(teachers, u)
		default:
			noGroup = append(noGroup, u)
		}
	}

	var sb strings.Builder

	fmt.Fprintf(&sb, "👥 Пользователи бота — %d чел.\n", len(users))

	groupIDs := make([]int, 0, len(groups))
	for id := range groups {
		groupIDs = append(groupIDs, id)
	}
	sort.Ints(groupIDs)

	for _, gID := range groupIDs {
		gData := groups[gID]

		total := 0
		for _, ul := range gData.bySubgroup {
			total += len(ul)
		}

		fmt.Fprintf(&sb, "\n🏫 Группа %d (%d чел.)\n", gID, total)

		subgroups := make([]int, 0, len(gData.bySubgroup))
		for sg := range gData.bySubgroup {
			subgroups = append(subgroups, sg)
		}
		sort.Ints(subgroups)

		for _, sg := range subgroups {
			ul := gData.bySubgroup[sg]
			label := subgroupLabel(sg)
			fmt.Fprintf(&sb, "  %s:\n", label)
			for _, u := range ul {
				fmt.Fprintf(&sb, "  • %s\n", formatUserMention(u))
			}
		}
	}

	if len(teachers) > 0 {
		fmt.Fprintf(&sb, "\n🎓 Преподаватели (%d чел.)\n", len(teachers))
		for _, u := range teachers {
			fmt.Fprintf(&sb, "  • %s — %s\n", formatUserMention(u), u.TeacherName)
		}
	}

	if len(noGroup) > 0 {
		fmt.Fprintf(&sb, "\n❓ Без группы (%d чел.)\n", len(noGroup))
		for _, u := range noGroup {
			fmt.Fprintf(&sb, "  • %s\n", formatUserMention(u))
		}
	}

	return sb.String()
}

func subgroupLabel(sg int) string {
	switch sg {
	case 0:
		return "Вся группа"
	case 1:
		return "1-я подгруппа"
	case 2:
		return "2-я подгруппа"
	default:
		return fmt.Sprintf("%d-я подгруппа", sg)
	}
}

func formatUserMention(u entities.User) string {
	if u.Username != "" {
		return "@" + u.Username
	}
	return fmt.Sprintf("id:%d (без юзернейма)", u.ChatID)
}
