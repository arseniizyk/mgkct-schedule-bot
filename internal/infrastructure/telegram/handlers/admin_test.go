package handlers_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
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
