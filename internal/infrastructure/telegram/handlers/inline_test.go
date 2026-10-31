package handlers_test

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockInlineContext struct {
	tele.Context

	query         *tele.Query
	queryResponse *tele.QueryResponse
}

func (c *mockInlineContext) Query() *tele.Query {
	return c.query
}

func (c *mockInlineContext) Answer(resp *tele.QueryResponse) error {
	c.queryResponse = resp
	return nil
}

type mockInlineScheduleService struct {
	groups       map[int32]*entities.Group
	weeksByGroup map[int32]entities.WeekNavigation
	scheduleByWk map[string]*entities.Group
	allWeeks     []time.Time
}

func (m *mockInlineScheduleService) GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error) {
	if g, ok := m.groups[groupID]; ok {
		return g, nil
	}
	return nil, fmt.Errorf("group not found")
}

func (m *mockInlineScheduleService) GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
	key := fmt.Sprintf("%d:%s", groupID, week.Format("2006-01-02"))
	if g, ok := m.scheduleByWk[key]; ok {
		return g, nil
	}
	if g, ok := m.groups[groupID]; ok {
		return g, nil
	}
	return nil, fmt.Errorf("schedule not found")
}

func (m *mockInlineScheduleService) GetAvailableWeeks(ctx context.Context, currentWeek time.Time) (entities.WeekNavigation, error) {
	for _, nav := range m.weeksByGroup {
		if nav.Current.Equal(currentWeek) {
			return nav, nil
		}
	}
	return entities.WeekNavigation{Current: currentWeek}, nil
}

func (m *mockInlineScheduleService) HasGroup(ctx context.Context, groupID int32) (bool, error) {
	_, ok := m.groups[groupID]
	return ok, nil
}

func (m *mockInlineScheduleService) GetAllWeeks(ctx context.Context) ([]time.Time, error) {
	if m.allWeeks != nil {
		return m.allWeeks, nil
	}
	return nil, nil
}

func (m *mockInlineScheduleService) GetArchiveSchedule(ctx context.Context, groupID int32, targetDate time.Time) (*entities.Group, error) {
	return nil, nil
}

type mockInlineTeacherService struct {
	teachers      map[string]*entities.Teacher
	weeksByTeach  map[string]entities.WeekNavigation
	scheduleByWk  map[string]*entities.Teacher
	candidatesMap map[string][]string
	allWeeks      []time.Time
}

func (m *mockInlineTeacherService) GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error) {
	if t, ok := m.teachers[name]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("teacher not found")
}

func (m *mockInlineTeacherService) GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	key := fmt.Sprintf("%s:%s", name, week.Format("2006-01-02"))
	if t, ok := m.scheduleByWk[key]; ok {
		return t, nil
	}
	if t, ok := m.teachers[name]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("schedule not found")
}

func (m *mockInlineTeacherService) GetAvailableWeeks(ctx context.Context, name string, currentWeek time.Time) (entities.WeekNavigation, error) {
	if nav, ok := m.weeksByTeach[name]; ok {
		return nav, nil
	}
	return entities.WeekNavigation{Current: currentWeek}, nil
}

func (m *mockInlineTeacherService) ValidateTeacher(ctx context.Context, name string) (string, []string, bool) {
	if _, ok := m.teachers[name]; ok {
		return name, nil, true
	}
	if cands, ok := m.candidatesMap[name]; ok {
		return "", cands, false
	}
	return "", nil, false
}

func (m *mockInlineTeacherService) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	names := make([]string, 0, len(m.teachers))
	for n := range m.teachers {
		names = append(names, n)
	}
	return names, nil
}

func (m *mockInlineTeacherService) GetAllWeeks(ctx context.Context, name string) ([]time.Time, error) {
	if m.allWeeks != nil {
		return m.allWeeks, nil
	}
	return nil, nil
}

func (m *mockInlineTeacherService) GetTeacherArchiveSchedule(ctx context.Context, name string, targetDate time.Time) (*entities.Teacher, error) {
	return nil, nil
}

func (m *mockInlineTeacherService) FindSimilarTeachers(ctx context.Context, name string, limit int) ([]string, error) {
	return nil, nil
}

func (m *mockInlineTeacherService) GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error) {
	return nil, nil
}

func TestInlineQuery(t *testing.T) {
	log := slog.Default()
	loc := time.FixedZone("MSK", 3*3600)
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, loc) // Monday
	nextWk := now.AddDate(0, 0, 7)
	prevWk := now.AddDate(0, 0, -7)
	fixedClock := func() time.Time { return now }

	mockSched := &mockInlineScheduleService{
		groups: map[int32]*entities.Group{
			88: {
				ID:   88,
				Week: now,
				Days: []*entities.Day{
					{
						Name: "Понедельник | 26.10.2026",
						Subjects: []*entities.Subject{
							{
								Pairs: []*entities.Pair{
									{Name: "1. Математика", Group: "Иванов", Class: "101"},
								},
							},
						},
					},
				},
			},
		},
		weeksByGroup: map[int32]entities.WeekNavigation{
			88: {
				Current: now,
				Next:    nextWk,
				Prev:    prevWk,
			},
		},
		scheduleByWk: make(map[string]*entities.Group),
		allWeeks:     []time.Time{nextWk, now, prevWk},
	}

	mockTeacher := &mockInlineTeacherService{
		teachers: map[string]*entities.Teacher{
			"Кулецкая Ю. Н.": {
				Name: "Кулецкая Ю. Н.",
				Week: now,
				Days: []*entities.Day{
					{
						Name: "Понедельник | 26.10.2026",
						Subjects: []*entities.Subject{
							{
								Pairs: []*entities.Pair{
									{Name: "Информатика", Group: "88", Class: "202"},
								},
							},
						},
					},
				},
			},
		},
		weeksByTeach: map[string]entities.WeekNavigation{
			"Кулецкая Ю. Н.": {
				Current: now,
				Next:    nextWk,
			},
		},
		candidatesMap: map[string][]string{
			"Кулец": {"Кулецкая Ю. Н."},
		},
		allWeeks: []time.Time{nextWk, now},
	}

	t.Run("Empty query with no saved user data returns help cards", func(t *testing.T) {
		repo := newMockUserRepo()
		h := handlers.InlineQueryWithClock(log, mockSched, mockTeacher, repo, fixedClock)

		c := &mockInlineContext{
			query: &tele.Query{
				Text:   "",
				Sender: &tele.User{ID: 100},
			},
		}

		err := h(c)
		require.NoError(t, err)
		require.NotNil(t, c.queryResponse)
		assert.True(t, c.queryResponse.IsPersonal)
		assert.Len(t, c.queryResponse.Results, 2)

		art1, ok := c.queryResponse.Results[0].(*tele.ArticleResult)
		require.True(t, ok)
		assert.Contains(t, art1.Title, "Расписание группы")

		art2, ok := c.queryResponse.Results[1].(*tele.ArticleResult)
		require.True(t, ok)
		assert.Contains(t, art2.Title, "Расписание преподавателя")
	})

	t.Run("Empty query with saved group returns user group schedule weeks", func(t *testing.T) {
		repo := newMockUserRepo()
		_ = repo.SetUserGroup(context.Background(), 100, 88, 0)

		h := handlers.InlineQueryWithClock(log, mockSched, mockTeacher, repo, fixedClock)

		c := &mockInlineContext{
			query: &tele.Query{
				Text:   "   ",
				Sender: &tele.User{ID: 100},
			},
		}

		err := h(c)
		require.NoError(t, err)
		require.NotNil(t, c.queryResponse)
		assert.Len(t, c.queryResponse.Results, 3) // Next, Current, Prev (от самой новой к старой)

		art0 := c.queryResponse.Results[0].(*tele.ArticleResult)
		assert.Contains(t, art0.Title, "Следующая неделя")
		assert.Contains(t, art0.Description, "Группа 88")

		art1 := c.queryResponse.Results[1].(*tele.ArticleResult)
		assert.Contains(t, art1.Title, "Текущая неделя")
		assert.Contains(t, art1.Description, "Группа 88")
		assert.Contains(t, art1.Text, "Математика")

		art2 := c.queryResponse.Results[2].(*tele.ArticleResult)
		assert.Contains(t, art2.Title, "Предыдущая неделя")
		assert.Contains(t, art2.Description, "Группа 88")
	})

	t.Run("Empty query with saved teacher returns user teacher schedule weeks", func(t *testing.T) {
		repo := newMockUserRepo()
		_ = repo.SetTeacher(context.Background(), 200, "Кулецкая Ю. Н.")

		h := handlers.InlineQueryWithClock(log, mockSched, mockTeacher, repo, fixedClock)

		c := &mockInlineContext{
			query: &tele.Query{
				Text:   "",
				Sender: &tele.User{ID: 200},
			},
		}

		err := h(c)
		require.NoError(t, err)
		require.NotNil(t, c.queryResponse)
		assert.Len(t, c.queryResponse.Results, 2) // Next, Current (DESC)

		art0 := c.queryResponse.Results[0].(*tele.ArticleResult)
		assert.Contains(t, art0.Title, "Следующая неделя")
		assert.Equal(t, "Кулецкая Ю. Н.", art0.Description)

		art1 := c.queryResponse.Results[1].(*tele.ArticleResult)
		assert.Contains(t, art1.Title, "Текущая неделя")
		assert.Equal(t, "Кулецкая Ю. Н.", art1.Description)
		assert.Contains(t, art1.Text, "Информатика")
	})

	t.Run("Group query with valid group ID returns available weeks sorted DESC with single switch button", func(t *testing.T) {
		repo := newMockUserRepo()
		h := handlers.InlineQueryWithClock(log, mockSched, mockTeacher, repo, fixedClock)

		c := &mockInlineContext{
			query: &tele.Query{
				Text:   "88",
				Sender: &tele.User{ID: 300},
			},
		}

		err := h(c)
		require.NoError(t, err)
		require.NotNil(t, c.queryResponse)
		assert.Len(t, c.queryResponse.Results, 3)

		// Проверяем сортировку: сначала следующая (02.11), затем текущая (26.10), затем предыдущая (19.10)
		art0 := c.queryResponse.Results[0].(*tele.ArticleResult)
		assert.Contains(t, art0.Title, "Следующая неделя (02.11 – 07.11)")
		assert.Equal(t, "Группа 88", art0.Description)
		require.NotNil(t, art0.ReplyMarkup)
		require.NotEmpty(t, art0.ReplyMarkup.InlineKeyboard)

		// Под сообщением ровно одна кнопка "Другое расписание"
		require.Len(t, art0.ReplyMarkup.InlineKeyboard[0], 1)
		assert.Equal(t, "🔍 Другое расписание", art0.ReplyMarkup.InlineKeyboard[0][0].Text)

		art1 := c.queryResponse.Results[1].(*tele.ArticleResult)
		assert.Contains(t, art1.Title, "Текущая неделя (26.10 – 31.10)")

		art2 := c.queryResponse.Results[2].(*tele.ArticleResult)
		assert.Contains(t, art2.Title, "Предыдущая неделя (19.10 – 24.10)")
	})

	t.Run("Group query with many weeks respects maxWeeks limit of 4", func(t *testing.T) {
		repo := newMockUserRepo()
		manyWeeksSched := &mockInlineScheduleService{
			groups: mockSched.groups,
			allWeeks: []time.Time{
				now.AddDate(0, 0, 14),
				now.AddDate(0, 0, 7),
				now,
				now.AddDate(0, 0, -7),
				now.AddDate(0, 0, -14),
				now.AddDate(0, 0, -21),
			},
		}

		h := handlers.InlineQueryWithClock(log, manyWeeksSched, mockTeacher, repo, fixedClock)
		c := &mockInlineContext{
			query: &tele.Query{
				Text:   "88",
				Sender: &tele.User{ID: 300},
			},
		}

		err := h(c)
		require.NoError(t, err)
		require.NotNil(t, c.queryResponse)
		assert.Len(t, c.queryResponse.Results, 4) // maxWeeks = 4
	})

	t.Run("Group query with subgroup returns formatted text for subgroup", func(t *testing.T) {
		repo := newMockUserRepo()
		h := handlers.InlineQueryWithClock(log, mockSched, mockTeacher, repo, fixedClock)

		c := &mockInlineContext{
			query: &tele.Query{
				Text:   "88 2",
				Sender: &tele.User{ID: 300},
			},
		}

		err := h(c)
		require.NoError(t, err)
		require.NotNil(t, c.queryResponse)
		assert.Len(t, c.queryResponse.Results, 3)

		art0 := c.queryResponse.Results[0].(*tele.ArticleResult)
		assert.Contains(t, art0.Description, "(2 п/г)")
	})

	t.Run("Group query with unknown group returns not found card", func(t *testing.T) {
		repo := newMockUserRepo()
		h := handlers.InlineQueryWithClock(log, mockSched, mockTeacher, repo, fixedClock)

		c := &mockInlineContext{
			query: &tele.Query{
				Text:   "999",
				Sender: &tele.User{ID: 300},
			},
		}

		err := h(c)
		require.NoError(t, err)
		require.NotNil(t, c.queryResponse)
		assert.Len(t, c.queryResponse.Results, 1)

		art0 := c.queryResponse.Results[0].(*tele.ArticleResult)
		assert.Contains(t, art0.Title, "Группа 999 не найдена")
	})

	t.Run("Teacher query with exact match returns weeks", func(t *testing.T) {
		repo := newMockUserRepo()
		h := handlers.InlineQueryWithClock(log, mockSched, mockTeacher, repo, fixedClock)

		c := &mockInlineContext{
			query: &tele.Query{
				Text:   "Кулецкая Ю. Н.",
				Sender: &tele.User{ID: 400},
			},
		}

		err := h(c)
		require.NoError(t, err)
		require.NotNil(t, c.queryResponse)
		assert.Len(t, c.queryResponse.Results, 2)

		art0 := c.queryResponse.Results[0].(*tele.ArticleResult)
		assert.Contains(t, art0.Title, "Следующая неделя")
		assert.Equal(t, "Кулецкая Ю. Н.", art0.Description)

		art1 := c.queryResponse.Results[1].(*tele.ArticleResult)
		assert.Contains(t, art1.Title, "Текущая неделя")
		assert.Equal(t, "Кулецкая Ю. Н.", art1.Description)
	})

	t.Run("Teacher query with candidate match limits to maxCandidates 10", func(t *testing.T) {
		repo := newMockUserRepo()
		manyCandidates := make([]string, 15)
		for i := range 15 {
			manyCandidates[i] = fmt.Sprintf("Преподаватель %d", i+1)
		}
		candTeacher := &mockInlineTeacherService{
			teachers: map[string]*entities.Teacher{},
			candidatesMap: map[string][]string{
				"Преп": manyCandidates,
			},
		}
		for _, cand := range manyCandidates {
			candTeacher.teachers[cand] = &entities.Teacher{
				Name: cand,
				Week: now,
			}
		}

		h := handlers.InlineQueryWithClock(log, mockSched, candTeacher, repo, fixedClock)
		c := &mockInlineContext{
			query: &tele.Query{
				Text:   "Преп",
				Sender: &tele.User{ID: 400},
			},
		}

		err := h(c)
		require.NoError(t, err)
		require.NotNil(t, c.queryResponse)
		assert.Len(t, c.queryResponse.Results, 10) // capped at maxCandidates = 10
	})

	t.Run("Teacher query with unknown teacher returns not found card", func(t *testing.T) {
		repo := newMockUserRepo()
		h := handlers.InlineQueryWithClock(log, mockSched, mockTeacher, repo, fixedClock)

		c := &mockInlineContext{
			query: &tele.Query{
				Text:   "Неизвестный",
				Sender: &tele.User{ID: 400},
			},
		}

		err := h(c)
		require.NoError(t, err)
		require.NotNil(t, c.queryResponse)
		assert.Len(t, c.queryResponse.Results, 1)

		art0 := c.queryResponse.Results[0].(*tele.ArticleResult)
		assert.Contains(t, art0.Title, "Преподаватель не найден")
	})
}
