package handlers_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/messages"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTeacherSvcForSuggestions struct {
	names    []string
	schedule map[string]*entities.Teacher
	member   map[string]*entities.FacultyMember
}

func (m *mockTeacherSvcForSuggestions) GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error) {
	if t, ok := m.schedule[name]; ok {
		return t, nil
	}
	return &entities.Teacher{Name: name, Week: time.Now()}, nil
}

func (m *mockTeacherSvcForSuggestions) GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	return m.GetTeacherLatestSchedule(ctx, name)
}

func (m *mockTeacherSvcForSuggestions) GetAvailableWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error) {
	return entities.WeekNavigation{}, nil
}

func (m *mockTeacherSvcForSuggestions) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	return m.names, nil
}

func (m *mockTeacherSvcForSuggestions) GetAllWeeks(ctx context.Context, name string) ([]time.Time, error) {
	return nil, nil
}

func (m *mockTeacherSvcForSuggestions) GetTeacherArchiveSchedule(ctx context.Context, name string, targetDate time.Time) (*entities.Teacher, error) {
	return nil, nil
}

func (m *mockTeacherSvcForSuggestions) ValidateTeacher(ctx context.Context, name string) (string, []string, bool) {
	for _, n := range m.names {
		if n == name {
			return n, nil, true
		}
	}
	return "", nil, false
}

func (m *mockTeacherSvcForSuggestions) FindSimilarTeachers(ctx context.Context, name string, limit int) ([]string, error) {
	if name == "Анципав" {
		return []string{"Анципов Е. Ю."}, nil
	}
	if name == "Иваноф" {
		return []string{"Иванов И. И.", "Иванова А. А."}, nil
	}
	return nil, nil
}

func (m *mockTeacherSvcForSuggestions) GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error) {
	if m.member != nil {
		return m.member[teacherName], nil
	}
	return nil, nil
}

func TestTeacher_TypoSuggestions(t *testing.T) {
	log := slog.Default()
	tSvc := &mockTeacherSvcForSuggestions{
		names: []string{"Анципов Е. Ю.", "Иванов И. И.", "Иванова А. А."},
		schedule: map[string]*entities.Teacher{
			"Анципов Е. Ю.": {Name: "Анципов Е. Ю.", Week: time.Now()},
		},
	}
	sm := state.NewMemoryManager()

	t.Run("Teachers command with typo suggests keyboard", func(t *testing.T) {
		h := handlers.Teachers(log, tSvc, sm)
		c := &mockContext{
			chatID: 123,
			args:   []string{"Анципав"},
		}

		err := h(c)
		require.NoError(t, err)
		assert.Equal(t, messages.TeacherNotFoundSuggestions, c.sentWhat)
		require.NotEmpty(t, c.sentOpts)

		var markup *tele.ReplyMarkup
		for _, opt := range c.sentOpts {
			if m, ok := opt.(*tele.ReplyMarkup); ok {
				markup = m
				break
			}
		}
		require.NotNil(t, markup)
		require.Len(t, markup.InlineKeyboard, 1)
		assert.Equal(t, "Анципов Е. Ю.", markup.InlineKeyboard[0][0].Text)
		assert.Equal(t, keyboard.TeacherCurrentWeek, markup.InlineKeyboard[0][0].Unique)
	})

	t.Run("Teachers command with unknown name returns not found", func(t *testing.T) {
		h := handlers.Teachers(log, tSvc, sm)
		c := &mockContext{
			chatID: 123,
			args:   []string{"Абракадабра"},
		}

		err := h(c)
		require.NoError(t, err)
		assert.Equal(t, messages.TeacherNotFound, c.sentWhat)
	})

	t.Run("WaitingTeacherSchedule state with typo suggests keyboard", func(t *testing.T) {
		_ = sm.Set(123, state.WaitingTeacherSchedule)
		h := handlers.StatesHandler(log, newMockUserRepo(), sm, tSvc)
		c := &mockContext{
			chatID: 123,
			text:   "Анципав",
		}

		err := h(c)
		require.NoError(t, err)
		assert.Equal(t, messages.TeacherNotFoundSuggestions, c.sentWhat)

		var markup *tele.ReplyMarkup
		for _, opt := range c.sentOpts {
			if m, ok := opt.(*tele.ReplyMarkup); ok {
				markup = m
				break
			}
		}
		require.NotNil(t, markup)
		assert.Equal(t, "Анципов Е. Ю.", markup.InlineKeyboard[0][0].Text)
	})

	t.Run("TeacherDay command with typo suggests keyboard", func(t *testing.T) {
		h := handlers.TeacherDay(log, tSvc, newMockUserRepo())
		c := &mockContext{
			chatID: 123,
			args:   []string{"Анципав"},
		}

		err := h(c)
		require.NoError(t, err)
		assert.Equal(t, messages.TeacherNotFoundSuggestions, c.sentWhat)
	})
}
