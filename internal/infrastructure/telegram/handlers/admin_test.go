package handlers_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
)

type mockAdminUserRepo struct {
	mockUserRepo

	users []entities.User
}

func (m *mockAdminUserRepo) AllUsersWithUsernames(ctx context.Context) ([]entities.User, error) {
	return m.users, nil
}

func (m *mockAdminUserRepo) AllUsers(ctx context.Context) ([]entities.User, error) {
	return m.users, nil
}

func (m *mockAdminUserRepo) CountUsers(ctx context.Context) (int, error) {
	return len(m.users), nil
}

func TestIsAdmin(t *testing.T) {
	adminIDs := []int64{123, 456, 789}

	assert.True(t, handlers.IsAdmin(adminIDs, 123))
	assert.True(t, handlers.IsAdmin(adminIDs, 456))
	assert.False(t, handlers.IsAdmin(adminIDs, 999))
	assert.False(t, handlers.IsAdmin(nil, 123))
}

func TestAdminOnly_Middleware(t *testing.T) {
	adminIDs := []int64{100}
	mw := handlers.AdminOnly(adminIDs)

	var nextCalled bool
	next := func(c tele.Context) error {
		nextCalled = true
		return nil
	}

	handler := mw(next)

	// Не администратор
	cNonAdmin := &mockContext{chatID: 200}
	err := handler(cNonAdmin)
	require.NoError(t, err)
	assert.False(t, nextCalled)
	assert.Equal(t, messages.Help, cNonAdmin.sentWhat)

	// Администратор
	cAdmin := &mockContext{chatID: 100}
	err = handler(cAdmin)
	require.NoError(t, err)
	assert.True(t, nextCalled)
}

func TestHelp_AdminVsRegular(t *testing.T) {
	adminIDs := []int64{100}
	h := handlers.Help(adminIDs)

	// Обычный пользователь
	cUser := &mockContext{chatID: 200}
	err := h(cUser)
	require.NoError(t, err)
	assert.Equal(t, messages.Help, cUser.sentWhat)
	assert.NotContains(t, cUser.sentWhat.(string), "Команды администратора")

	// Администратор
	cAdmin := &mockContext{chatID: 100}
	err = h(cAdmin)
	require.NoError(t, err)
	assert.Contains(t, cAdmin.sentWhat.(string), "Команды администратора")
	assert.Contains(t, cAdmin.sentWhat.(string), "/userstats")
	assert.Contains(t, cAdmin.sentWhat.(string), "/broadcast")
	assert.Contains(t, cAdmin.sentWhat.(string), "/botinfo")
}

func TestUserStats(t *testing.T) {
	repo := &mockAdminUserRepo{
		users: []entities.User{
			{ChatID: 1, Username: "Ahhpixel", Group: 88, Subgroup: 0},
			{ChatID: 2, Username: "besanets", Group: 88, Subgroup: 1},
			{ChatID: 3, Username: "", Group: 88, Subgroup: 0},
			{ChatID: 4, Username: "epeoeoeoel", Group: 98, Subgroup: 2},
			{ChatID: 5, Username: "teacher1", TeacherName: "Агеенкова Д. Д."},
			{ChatID: 6, Username: "lonely_user"},
		},
	}

	h := handlers.UserStats(testLog(), repo)
	ctx := &mockContext{chatID: 100}

	err := h(ctx)
	require.NoError(t, err)

	out, ok := ctx.sentWhat.(string)
	require.True(t, ok)

	assert.Contains(t, out, "Пользователи бота — 6 чел.")
	assert.Contains(t, out, "🏫 Группа 88 (3 чел.)")
	assert.Contains(t, out, "Вся группа:")
	assert.Contains(t, out, "@Ahhpixel")
	assert.Contains(t, out, "id:3 (без юзернейма)")
	assert.Contains(t, out, "1-я подгруппа:")
	assert.Contains(t, out, "@besanets")
	assert.Contains(t, out, "🏫 Группа 98 (1 чел.)")
	assert.Contains(t, out, "2-я подгруппа:")
	assert.Contains(t, out, "@epeoeoeoel")
	assert.Contains(t, out, "🎓 Преподаватели (1 чел.)")
	assert.Contains(t, out, "@teacher1 — Агеенкова Д. Д.")
	assert.Contains(t, out, "❓ Без группы (1 чел.)")
	assert.Contains(t, out, "@lonely_user")
}

func TestBotInfo(t *testing.T) {
	repo := &mockAdminUserRepo{
		users: []entities.User{
			{ChatID: 1, Group: 88},
			{ChatID: 2, TeacherName: "Иванов И. И."},
			{ChatID: 3},
		},
	}

	h := handlers.BotInfo(testLog(), repo)
	ctx := &mockContext{chatID: 100}

	err := h(ctx)
	require.NoError(t, err)

	out, ok := ctx.sentWhat.(string)
	require.True(t, ok)
	assert.Contains(t, out, "Всего пользователей:</b> 3")
	assert.Contains(t, out, "Студентов (с группой): 1")
	assert.Contains(t, out, "Преподавателей: 1")
	assert.Contains(t, out, "Без группы: 1")
}

func TestBroadcast_CommandAndStates(t *testing.T) {
	sm := state.NewMemoryManager()
	h := handlers.Broadcast(testLog(), sm)
	ctx := &mockContext{chatID: 100}

	err := h(ctx)
	require.NoError(t, err)

	st, ok := sm.Get(100)
	assert.True(t, ok)
	assert.Equal(t, state.WaitingBroadcast, st)
	assert.Contains(t, ctx.sentWhat.(string), "Введите сообщение для рассылки")
}

type mockAdminBot struct {
	sentMessages []struct {
		to   tele.Recipient
		what any
	}
	mu sync.Mutex
}

func (m *mockAdminBot) Send(to tele.Recipient, what any, opts ...any) (*tele.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentMessages = append(m.sentMessages, struct {
		to   tele.Recipient
		what any
	}{to: to, what: what})
	return nil, nil
}

func (m *mockAdminBot) getSent() []struct {
	to   tele.Recipient
	what any
} {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]struct {
		to   tele.Recipient
		what any
	}, len(m.sentMessages))
	copy(copied, m.sentMessages)
	return copied
}

func TestBroadcast_PreviewAndValidation(t *testing.T) {
	sm := state.NewMemoryManager()
	adminIDs := []int64{100}
	bot := &mockAdminBot{}
	repo := &mockAdminUserRepo{}

	h := handlers.StatesHandler(testLog(), repo, sm, nil, nil, nil, nil, bot, adminIDs)

	// 1. Пустой текст
	require.NoError(t, sm.Set(100, state.WaitingBroadcast))
	cEmpty := &mockContext{chatID: 100, text: "   "}
	err := h(cEmpty)
	require.NoError(t, err)
	assert.Equal(t, "❌ Сообщение не может быть пустым.", cEmpty.sentWhat)

	// 2. Валидный текст
	require.NoError(t, sm.Set(100, state.WaitingBroadcast))
	cValid := &mockContext{chatID: 100, text: "Внимание, завтра занятий нет!"}
	err = h(cValid)
	require.NoError(t, err)

	// Состояние в sm сброшено
	_, hasState := sm.Get(100)
	assert.False(t, hasState)

	// Черновик сохранён
	draft, ok := handlers.GetBroadcastDraft(100)
	assert.True(t, ok)
	assert.Equal(t, "Внимание, завтра занятий нет!", draft)

	// Отправлен предпросмотр
	assert.Contains(t, cValid.sentWhat.(string), "Предпросмотр сообщения для рассылки:")
	assert.Contains(t, cValid.sentWhat.(string), "Внимание, завтра занятий нет!")
	assert.Contains(t, cValid.sentOpts, keyboard.InlineBroadcastKeyboard())
}

func TestBroadcast_CancelCallback(t *testing.T) {
	sm := state.NewMemoryManager()
	adminIDs := []int64{100}
	bot := &mockAdminBot{}
	repo := &mockAdminUserRepo{}

	cbHandler := handlers.CallbacksHandler(testLog(), nil, nil, nil, repo, sm, bot, adminIDs)

	handlers.SetBroadcastDraft(100, "Тестовое сообщение")

	// 1. Неадминистратор пытается отменить
	cNonAdmin := &mockContext{
		chatID:   200,
		callback: &tele.Callback{Data: "\f" + keyboard.BroadcastCancel},
	}
	err := cbHandler(cNonAdmin)
	require.NoError(t, err)
	require.NotNil(t, cNonAdmin.responded)
	assert.Equal(t, "Доступ запрещен", cNonAdmin.responded.Text)
	assert.Equal(t, 1, cNonAdmin.respondCalls)

	// Черновик администратора не должен быть удален
	_, ok := handlers.GetBroadcastDraft(100)
	assert.True(t, ok)

	// 2. Администратор отменяет
	cAdmin := &mockContext{
		chatID:   100,
		callback: &tele.Callback{Data: "\f" + keyboard.BroadcastCancel},
	}
	err = cbHandler(cAdmin)
	require.NoError(t, err)
	assert.Equal(t, "❌ Рассылка отменена.", cAdmin.editedWhat)
	assert.Equal(t, 1, cAdmin.respondCalls)

	_, ok = handlers.GetBroadcastDraft(100)
	assert.False(t, ok)
}

func TestBroadcast_ConfirmCallback(t *testing.T) {
	sm := state.NewMemoryManager()
	adminIDs := []int64{100}
	bot := &mockAdminBot{}
	repo := &mockAdminUserRepo{
		users: []entities.User{
			{ChatID: 201},
			{ChatID: 202},
		},
	}

	cbHandler := handlers.CallbacksHandler(testLog(), nil, nil, nil, repo, sm, bot, adminIDs)

	// 1. Неадминистратор пытается подтвердить
	cNonAdmin := &mockContext{
		chatID:   200,
		callback: &tele.Callback{Data: "\f" + keyboard.BroadcastConfirm},
	}
	err := cbHandler(cNonAdmin)
	require.NoError(t, err)
	require.NotNil(t, cNonAdmin.responded)
	assert.Equal(t, "Доступ запрещен", cNonAdmin.responded.Text)
	assert.Equal(t, 1, cNonAdmin.respondCalls)

	// 2. Администратор подтверждает, когда черновика нет
	handlers.ClearBroadcastDraft(100)
	cAdminNoDraft := &mockContext{
		chatID:   100,
		callback: &tele.Callback{Data: "\f" + keyboard.BroadcastConfirm},
	}
	err = cbHandler(cAdminNoDraft)
	require.NoError(t, err)
	assert.Contains(t, cAdminNoDraft.editedWhat, "не найдено или устарело")
	assert.Equal(t, 1, cAdminNoDraft.respondCalls)

	// 3. Администратор подтверждает с валидным черновиком
	handlers.SetBroadcastDraft(100, "Важное объявление!")
	cAdmin := &mockContext{
		chatID:   100,
		callback: &tele.Callback{Data: "\f" + keyboard.BroadcastConfirm},
	}
	err = cbHandler(cAdmin)
	require.NoError(t, err)
	assert.Equal(t, "🚀 Рассылка запущена...", cAdmin.editedWhat)
	assert.Equal(t, 1, cAdmin.respondCalls)

	// Черновик очищен
	_, ok := handlers.GetBroadcastDraft(100)
	assert.False(t, ok)

	// Ждем выполнения асинхронной рассылки
	assert.Eventually(t, func() bool {
		sent := bot.getSent()
		// Ожидаем 2 сообщения пользователям + 1 отчет администратору = 3
		return len(sent) == 3
	}, 2*time.Second, 20*time.Millisecond)

	sent := bot.getSent()
	assert.Equal(t, tele.ChatID(201), sent[0].to)
	assert.Equal(t, "Важное объявление!", sent[0].what)
	assert.Equal(t, tele.ChatID(202), sent[1].to)
	assert.Equal(t, "Важное объявление!", sent[1].what)
	assert.Equal(t, tele.ChatID(100), sent[2].to)
	assert.Contains(t, sent[2].what.(string), "Рассылка завершена")
}

func TestBroadcast_EmptyAdminIDs_Denied(t *testing.T) {
	sm := state.NewMemoryManager()
	bot := &mockAdminBot{}
	repo := &mockAdminUserRepo{}

	// Пустой список adminIDs
	cbHandler := handlers.CallbacksHandler(testLog(), nil, nil, nil, repo, sm, bot, []int64{})

	cConfirm := &mockContext{
		chatID:   100,
		callback: &tele.Callback{Data: "\f" + keyboard.BroadcastConfirm},
	}
	err := cbHandler(cConfirm)
	require.NoError(t, err)
	require.NotNil(t, cConfirm.responded)
	assert.Equal(t, "Доступ запрещен", cConfirm.responded.Text)
	assert.Equal(t, 1, cConfirm.respondCalls)

	cCancel := &mockContext{
		chatID:   100,
		callback: &tele.Callback{Data: "\f" + keyboard.BroadcastCancel},
	}
	err = cbHandler(cCancel)
	require.NoError(t, err)
	require.NotNil(t, cCancel.responded)
	assert.Equal(t, "Доступ запрещен", cCancel.responded.Text)
	assert.Equal(t, 1, cCancel.respondCalls)
}

func TestBroadcast_MessageTooLong(t *testing.T) {
	sm := state.NewMemoryManager()
	adminIDs := []int64{100}
	bot := &mockAdminBot{}
	repo := &mockAdminUserRepo{}

	h := handlers.StatesHandler(testLog(), repo, sm, nil, nil, nil, nil, bot, adminIDs)

	require.NoError(t, sm.Set(100, state.WaitingBroadcast))
	longText := make([]rune, 4001)
	for i := range longText {
		longText[i] = 'A'
	}
	cLong := &mockContext{chatID: 100, text: string(longText)}
	err := h(cLong)
	require.NoError(t, err)
	assert.Contains(t, cLong.sentWhat.(string), "слишком длинное")

	// Черновик не должен быть сохранён
	_, ok := handlers.GetBroadcastDraft(100)
	assert.False(t, ok)
}

func TestBroadcast_ContextCancellation(t *testing.T) {
	sm := state.NewMemoryManager()
	adminIDs := []int64{100}
	bot := &mockAdminBot{}
	repo := &mockAdminUserRepo{
		users: []entities.User{
			{ChatID: 201},
			{ChatID: 202},
			{ChatID: 203},
		},
	}

	cancelCtx, cancel := context.WithCancel(context.Background())
	handlers.SetBroadcastContext(cancelCtx)
	defer handlers.SetBroadcastContext(context.Background())

	cbHandler := handlers.CallbacksHandler(testLog(), nil, nil, nil, repo, sm, bot, adminIDs)

	handlers.SetBroadcastDraft(100, "Отменяемое сообщение")
	cAdmin := &mockContext{
		chatID:   100,
		callback: &tele.Callback{Data: "\f" + keyboard.BroadcastConfirm},
	}
	err := cbHandler(cAdmin)
	require.NoError(t, err)

	// Отменяем контекст немедленно
	cancel()

	assert.Eventually(t, func() bool {
		sent := bot.getSent()
		for _, s := range sent {
			if s.to == tele.ChatID(100) {
				if str, ok := s.what.(string); ok && assert.ObjectsAreEqual(ok, true) {
					if strings.Contains(str, "прервана") || strings.Contains(str, "завершена") {
						return true
					}
				}
			}
		}
		return false
	}, 2*time.Second, 20*time.Millisecond)
}
