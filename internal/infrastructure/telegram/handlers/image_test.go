package handlers_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/renderer"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
)

type mockAPI struct {
	tele.API

	sentWhat any
}

func (m *mockAPI) Send(to tele.Recipient, what any, opts ...any) (*tele.Message, error) {
	m.sentWhat = what
	return &tele.Message{
		ID: 101,
		Photo: &tele.Photo{
			FileID: "telegram_file_id_test_123",
		},
	}, nil
}

type mockImageTeleContext struct {
	tele.Context

	chat      *tele.Chat
	data      string
	sentWhat  any
	sentOpts  []any
	responded bool
	args      []string
	api       *mockAPI
}

func (m *mockImageTeleContext) Chat() *tele.Chat {
	if m.chat != nil {
		return m.chat
	}
	return &tele.Chat{ID: 12345}
}

func (m *mockImageTeleContext) Recipient() tele.Recipient {
	return m.Chat()
}

func (m *mockImageTeleContext) Data() string {
	return m.data
}

func (m *mockImageTeleContext) Callback() *tele.Callback {
	return &tele.Callback{
		Data: m.data,
	}
}

func (m *mockImageTeleContext) Send(what any, opts ...any) error {
	m.sentWhat = what
	m.sentOpts = opts
	return nil
}

func (m *mockImageTeleContext) Respond(resp ...*tele.CallbackResponse) error {
	m.responded = true
	return nil
}

func (m *mockImageTeleContext) Args() []string {
	return m.args
}

func (m *mockImageTeleContext) Bot() tele.API {
	return m.api
}

func TestHandleWeekImageCallback(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	renderer.GetDefaultCache().Clear()

	weekDate := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	group := &entities.Group{
		ID:   80,
		Week: weekDate,
		Days: []*entities.Day{
			{
				Name: "Понедельник | 02.11.2026",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "НЭОПИМ", Type: "Лек", Class: "3-114", Group: "Камлюк В. С."},
						},
					},
				},
			},
		},
	}

	schedSvc := &mockNavScheduleSvc{
		latestGroup: group,
	}
	teacherSvc := &mockTeacherService{}
	userRepo := newMockUserRepo()
	_ = userRepo.SetUserGroup(context.Background(), 12345, 80, 0)

	api := &mockAPI{}
	cbHandler := handlers.CallbacksHandler(log, schedSvc, teacherSvc, nil, userRepo, nil, api, []int64{1})

	// 1-й запрос: генерация и сохранение FileID в кэш
	c1 := &mockImageTeleContext{
		data: fmt.Sprintf("%s|80:current:0", keyboard.WeekImage),
		api:  api,
	}

	err := cbHandler(c1)
	require.NoError(t, err)
	assert.True(t, c1.responded)
	assert.NotNil(t, api.sentWhat)
	photo, ok := api.sentWhat.(*tele.Photo)
	require.True(t, ok)
	assert.NotNil(t, photo.File.FileReader)

	// Проверяем, что кэш запомнил FileID
	cacheKey := renderer.GroupKey(80, weekDate, 0)
	cachedID, found := renderer.GetDefaultCache().GetFileID(cacheKey)
	assert.True(t, found)
	assert.Equal(t, "telegram_file_id_test_123", cachedID)

	// 2-й запрос: отдача из кэша по FileID
	c2 := &mockImageTeleContext{
		data: fmt.Sprintf("%s|80:current:0", keyboard.WeekImage),
		api:  api,
	}
	err = cbHandler(c2)
	require.NoError(t, err)
	assert.True(t, c2.responded)
	cachedPhoto, ok := c2.sentWhat.(*tele.Photo)
	require.True(t, ok)
	assert.Equal(t, "telegram_file_id_test_123", cachedPhoto.File.FileID)
}

func TestScheduleImageCommand(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	renderer.GetDefaultCache().Clear()

	weekDate := time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC)
	group := &entities.Group{
		ID:   80,
		Week: weekDate,
		Days: []*entities.Day{
			{
				Name: "Понедельник | 02.11.2026",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "НЭОПИМ", Type: "Лек", Class: "3-114", Group: "Камлюк В. С."},
						},
					},
				},
			},
		},
	}

	schedSvc := &mockNavScheduleSvc{
		latestGroup: group,
	}
	teacherSvc := &mockTeacherService{}
	userRepo := newMockUserRepo()
	_ = userRepo.SetUserGroup(context.Background(), 12345, 80, 1)

	imgCmdHandler := handlers.ScheduleImage(log, schedSvc, teacherSvc, userRepo)

	// Вызов /pic без аргументов (пользователь студент группы 80, подгруппы 1)
	api := &mockAPI{}
	c := &mockImageTeleContext{
		args: nil,
		api:  api,
	}
	err := imgCmdHandler(c)
	require.NoError(t, err)
	require.NotNil(t, api.sentWhat)
	photo, ok := api.sentWhat.(*tele.Photo)
	require.True(t, ok)
	assert.NotNil(t, photo.File.FileReader)

	// Проверяем сохранение в кэш
	cacheKey := renderer.GroupKey(80, weekDate, 1)
	cachedID, found := renderer.GetDefaultCache().GetFileID(cacheKey)
	assert.True(t, found)
	assert.Equal(t, "telegram_file_id_test_123", cachedID)
}

func TestScheduleImageCommand_Teacher(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	renderer.GetDefaultCache().Clear()

	teacher := &entities.Teacher{
		Name: "Камлюк В. С.",
		Week: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		Days: []*entities.Day{
			{
				Name: "Понедельник | 02.11.2026",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "НЭОПИМ", Type: "Лек", Class: "3-114", Group: "80"},
						},
					},
				},
			},
		},
	}

	schedSvc := &mockNavScheduleSvc{}
	teacherSvc := &mockTeacherService{
		teacher: teacher,
	}
	userRepo := newMockUserRepo()

	imgCmdHandler := handlers.ScheduleImage(log, schedSvc, teacherSvc, userRepo)

	t.Run("valid teacher query", func(t *testing.T) {
		api := &mockAPI{}
		c := &mockImageTeleContext{
			args: []string{"Камлюк"},
			api:  api,
		}
		err := imgCmdHandler(c)
		require.NoError(t, err)
		require.NotNil(t, api.sentWhat)
		photo, ok := api.sentWhat.(*tele.Photo)
		require.True(t, ok)
		assert.NotNil(t, photo.File.FileReader)

		// Кэш FileID
		cacheKey := renderer.TeacherKey(teacher.Name, teacher.Week)
		cachedID, found := renderer.GetDefaultCache().GetFileID(cacheKey)
		assert.True(t, found)
		assert.Equal(t, "telegram_file_id_test_123", cachedID)
	})

	t.Run("invalid teacher query", func(t *testing.T) {
		emptyTeacherSvc := &mockTeacherService{teacher: nil}
		emptyHandler := handlers.ScheduleImage(log, schedSvc, emptyTeacherSvc, userRepo)

		c := &mockImageTeleContext{
			args: []string{"Несуществующий"},
			api:  &mockAPI{},
		}
		err := emptyHandler(c)
		require.NoError(t, err)
		assert.Contains(t, c.sentWhat, "не найден")
	})
}

func TestHandleTeacherWeekImageCallback(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	renderer.GetDefaultCache().Clear()

	teacher := &entities.Teacher{
		Name: "Камлюк В. С.",
		Week: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		Days: []*entities.Day{
			{
				Name: "Понедельник | 02.11.2026",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "НЭОПИМ", Type: "Лек", Class: "3-114", Group: "80"},
						},
					},
				},
			},
		},
	}

	schedSvc := &mockNavScheduleSvc{}
	teacherSvc := &mockTeacherService{
		teacher: teacher,
	}
	userRepo := newMockUserRepo()

	api := &mockAPI{}
	cbHandler := handlers.CallbacksHandler(log, schedSvc, teacherSvc, nil, userRepo, nil, api, []int64{1})

	// 1-й вызов: генерация и кэширование
	c1 := &mockImageTeleContext{
		data: fmt.Sprintf("%s|Камлюк В. С.:current", keyboard.TeacherWeekImage),
		api:  api,
	}

	err := cbHandler(c1)
	require.NoError(t, err)
	assert.True(t, c1.responded)
	assert.NotNil(t, api.sentWhat)
	photo, ok := api.sentWhat.(*tele.Photo)
	require.True(t, ok)
	assert.NotNil(t, photo.File.FileReader)

	// Проверяем кэш FileID
	cacheKey := renderer.TeacherKey(teacher.Name, teacher.Week)
	cachedID, found := renderer.GetDefaultCache().GetFileID(cacheKey)
	assert.True(t, found)
	assert.Equal(t, "telegram_file_id_test_123", cachedID)

	// 2-й вызов: отдача из кэша
	c2 := &mockImageTeleContext{
		data: fmt.Sprintf("%s|Камлюк В. С.:current", keyboard.TeacherWeekImage),
		api:  api,
	}
	err = cbHandler(c2)
	require.NoError(t, err)
	assert.True(t, c2.responded)
	cachedPhoto, ok := c2.sentWhat.(*tele.Photo)
	require.True(t, ok)
	assert.Equal(t, "telegram_file_id_test_123", cachedPhoto.File.FileID)
}
