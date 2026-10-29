package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/formatter"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
)

// BotSender — минимальный интерфейс для отправки сообщений от имени бота (используется при рассылке).
type BotSender interface {
	Send(to tele.Recipient, what any, opts ...any) (*tele.Message, error)
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
		var withGroup, withTeacher, noGroup int
		for _, u := range users {
			switch {
			case u.Group > 0:
				withGroup++
			case u.TeacherName != "":
				withTeacher++
			default:
				noGroup++
			}
		}

		minskLoc := time.FixedZone("MSK", 3*3600)
		now := time.Now().In(minskLoc)

		msg := fmt.Sprintf(
			"ℹ️ <b>Информация о боте</b>\n\n"+
				"👥 <b>Всего пользователей:</b> %d\n"+
				"  • Студентов (с группой): %d\n"+
				"  • Преподавателей: %d\n"+
				"  • Без группы: %d\n\n"+
				"⏰ <b>Время сервера:</b> %s MSK",
			total, withGroup, withTeacher, noGroup,
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

//nolint:fatcontext // broadcastContext хранит глобальный контекст жизненного цикла бота для фоновых рассылок
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
	text, ok := broadcastDrafts[chatID]
	return text, ok
}

func ClearBroadcastDraft(chatID int64) {
	broadcastDraftsMu.Lock()
	defer broadcastDraftsMu.Unlock()
	delete(broadcastDrafts, chatID)
}

// handleWaitingBroadcast обрабатывает ввод сообщения для рассылки.
// Показывает администратору предпросмотр сообщения с инлайн-кнопками «✅ Отправить» и «❌ Отмена».
func handleWaitingBroadcast(
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

// handleBroadcastConfirm подтверждает и запускает рассылку сообщений всем пользователям в отдельной горутине.
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

	// Запуск рассылки в отдельной горутине с контекстом жизненного цикла бота
	go executeBroadcast(getBroadcastContext(), log, chatID, text, userRepo, bot)

	return nil
}

// handleBroadcastCancel отменяет рассылку сообщений.
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

	throttle := time.NewTicker(40 * time.Millisecond)
	defer throttle.Stop()

	var sent, failed int
	for _, u := range users {
		select {
		case <-ctx.Done():
			log.Warn("broadcast cancelled by context")
			_, _ = bot.Send(tele.ChatID(adminChatID), fmt.Sprintf("⚠️ Рассылка прервана. Отправлено: %d / Ошибок: %d", sent, failed))
			return
		case <-throttle.C:
			if _, sendErr := bot.Send(tele.ChatID(u.ChatID), text); sendErr != nil {
				log.Warn("broadcast: failed sending to user", "chat_id", u.ChatID, "error", sendErr)
				failed++
			} else {
				sent++
			}
		}
	}

	_, _ = bot.Send(tele.ChatID(adminChatID), fmt.Sprintf("✅ Рассылка завершена.\nОтправлено: %d\nОшибок: %d", sent, failed))
}

// formatUserStats формирует текст статистики пользователей по группам и подгруппам.
func formatUserStats(users []entities.User) string {
	// Разделяем пользователей по категориям
	type groupData struct {
		bySubgroup map[int][]entities.User // subgroup -> users (0 = вся группа)
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

	// Сортируем группы по номеру
	groupIDs := make([]int, 0, len(groups))
	for id := range groups {
		groupIDs = append(groupIDs, id)
	}
	sort.Ints(groupIDs)

	for _, gID := range groupIDs {
		gData := groups[gID]

		// Подсчёт суммарного количества в группе
		total := 0
		for _, ul := range gData.bySubgroup {
			total += len(ul)
		}

		fmt.Fprintf(&sb, "\n🏫 Группа %d (%d чел.)\n", gID, total)

		// Сортируем подгруппы: 0 (вся группа) первым
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

	// Преподаватели
	if len(teachers) > 0 {
		fmt.Fprintf(&sb, "\n🎓 Преподаватели (%d чел.)\n", len(teachers))
		for _, u := range teachers {
			fmt.Fprintf(&sb, "  • %s — %s\n", formatUserMention(u), u.TeacherName)
		}
	}

	// Без группы
	if len(noGroup) > 0 {
		fmt.Fprintf(&sb, "\n❓ Без группы (%d чел.)\n", len(noGroup))
		for _, u := range noGroup {
			fmt.Fprintf(&sb, "  • %s\n", formatUserMention(u))
		}
	}

	return sb.String()
}

// subgroupLabel возвращает текстовую метку подгруппы.
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

// formatUserMention возвращает @username или id:<chatID> если username отсутствует.
func formatUserMention(u entities.User) string {
	if u.Username != "" {
		return "@" + u.Username
	}
	return fmt.Sprintf("id:%d (без юзернейма)", u.ChatID)
}
