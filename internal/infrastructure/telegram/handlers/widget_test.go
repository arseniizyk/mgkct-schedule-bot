package handlers_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/pkg/timeutil"
)

type mockWidgetContext struct {
	tele.Context

	chat     *tele.Chat
	args     []string
	sentWhat any
	sentOpts []any
}

func (m *mockWidgetContext) Chat() *tele.Chat {
	return m.chat
}

func (m *mockWidgetContext) Recipient() tele.Recipient {
	return m.chat
}

func (m *mockWidgetContext) Args() []string {
	return m.args
}

func (m *mockWidgetContext) Bot() tele.API {
	return nil
}

func (m *mockWidgetContext) Send(what any, opts ...any) error {
	m.sentWhat = what
	m.sentOpts = opts
	return nil
}

type mockMTProtoSender struct {
	ready      bool
	sentChatID int64
	sentHTML   string
}

func (m *mockMTProtoSender) IsReady() bool {
	return m.ready
}

func (m *mockMTProtoSender) SendWidget(ctx context.Context, chatID int64, html, caption string) error {
	m.sentChatID = chatID
	m.sentHTML = html
	return nil
}

func TestWidgetHandler_GroupArg_WithMTProto(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	week := time.Date(2026, 11, 9, 0, 0, 0, 0, timeutil.MinskLocation)

	group := &entities.Group{
		ID:   81,
		Week: week,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Математика", Class: "101"},
						},
					},
				},
			},
		},
	}

	schedSvc := &mockCalendarScheduleService{group: group}
	teacherSvc := &mockCalendarTeacherService{}
	userRepo := newMockUserRepo()
	mtproto := &mockMTProtoSender{ready: true}

	handler := handlers.Widget(logger, schedSvc, teacherSvc, userRepo, "https://schedule.mgkct.by", mtproto)

	ctx := &mockWidgetContext{
		chat: &tele.Chat{ID: 12345},
		args: []string{"81"},
	}

	err := handler(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(12345), mtproto.sentChatID)
	assert.Contains(t, mtproto.sentHTML, "Группа 81")
	assert.Nil(t, ctx.sentWhat, "Context.Send should not be called when MTProto succeeds")
}

func TestWidgetHandler_TeacherArg(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	week := time.Date(2026, 11, 9, 0, 0, 0, 0, timeutil.MinskLocation)

	teacher := &entities.Teacher{
		Name: "Иванов И.И.",
		Week: week,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Физика", Class: "202", Group: "81"},
						},
					},
				},
			},
		},
	}

	schedSvc := &mockCalendarScheduleService{}
	teacherSvc := &mockCalendarTeacherService{teacher: teacher}
	userRepo := newMockUserRepo()

	handler := handlers.Widget(logger, schedSvc, teacherSvc, userRepo, "https://schedule.mgkct.by", nil)

	ctx := &mockWidgetContext{
		chat: &tele.Chat{ID: 12345},
		args: []string{"Иванов"},
	}

	err := handler(ctx)
	require.NoError(t, err)
	assert.NotNil(t, ctx.sentWhat)
	assert.Contains(t, ctx.sentWhat, "Иванов И.И.")
}

func TestWidgetHandler_UserProfileFallback(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	week := time.Date(2026, 11, 9, 0, 0, 0, 0, timeutil.MinskLocation)

	group := &entities.Group{
		ID:   81,
		Week: week,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Информатика", Class: "303"},
						},
					},
				},
			},
		},
	}

	schedSvc := &mockCalendarScheduleService{group: group}
	teacherSvc := &mockCalendarTeacherService{}
	userRepo := newMockUserRepo()
	userRepo.userGroup[12345] = 81
	userRepo.userSubgroup[12345] = 1

	handler := handlers.Widget(logger, schedSvc, teacherSvc, userRepo, "", nil)

	ctx := &mockWidgetContext{
		chat: &tele.Chat{ID: 12345},
		args: []string{},
	}

	err := handler(ctx)
	require.NoError(t, err)
	assert.NotNil(t, ctx.sentWhat)
	assert.Contains(t, ctx.sentWhat, "Группа 81")
}

func TestWidgetHandler_GroupChat_Rejected(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	schedSvc := &mockCalendarScheduleService{}
	teacherSvc := &mockCalendarTeacherService{}
	userRepo := newMockUserRepo()

	handler := handlers.Widget(logger, schedSvc, teacherSvc, userRepo, "", nil)

	ctx := &mockWidgetContext{
		chat: &tele.Chat{ID: -1001234567890, Type: tele.ChatSuperGroup},
		args: []string{"81"},
	}

	err := handler(ctx)
	require.NoError(t, err)
	assert.Contains(t, ctx.sentWhat, "Интерактивные виджеты поддерживаются Telegram только в личных сообщениях")
}
