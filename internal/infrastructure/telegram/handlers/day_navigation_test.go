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
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
)

type mockNavScheduleSvc struct {
	groupsByWeek map[string]*entities.Group // key = "groupID:2006-01-02"
	latestGroup  *entities.Group
	weeksNav     map[string]entities.WeekNavigation // key = "2006-01-02"
}

func (m *mockNavScheduleSvc) GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error) {
	if m.latestGroup != nil {
		return m.latestGroup, nil
	}
	return nil, fmt.Errorf("group not found")
}

func (m *mockNavScheduleSvc) GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
	key := fmt.Sprintf("%d:%s", groupID, week.Format("2006-01-02"))
	if g, ok := m.groupsByWeek[key]; ok {
		return g, nil
	}
	return nil, fmt.Errorf("group schedule for week %s not found", week.Format("2006-01-02"))
}

func (m *mockNavScheduleSvc) GetAvailableWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error) {
	key := week.Format("2006-01-02")
	if nav, ok := m.weeksNav[key]; ok {
		return nav, nil
	}
	return entities.WeekNavigation{}, nil
}

func (m *mockNavScheduleSvc) GetAllWeeks(ctx context.Context) ([]time.Time, error) {
	return nil, nil
}

func (m *mockNavScheduleSvc) GetArchiveSchedule(ctx context.Context, groupID int32, targetDate time.Time) (*entities.Group, error) {
	return nil, nil
}

func (m *mockNavScheduleSvc) HasGroup(ctx context.Context, groupID int32) (bool, error) {
	return true, nil
}

type mockNavTeacherSvc struct {
	teachersByWeek map[string]*entities.Teacher // key = "name:2006-01-02"
	latestTeacher  *entities.Teacher
	weeksNav       map[string]entities.WeekNavigation // key = "2006-01-02"
}

func (m *mockNavTeacherSvc) GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error) {
	if m.latestTeacher != nil {
		return m.latestTeacher, nil
	}
	return nil, fmt.Errorf("teacher not found")
}

func (m *mockNavTeacherSvc) GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	key := fmt.Sprintf("%s:%s", name, week.Format("2006-01-02"))
	if t, ok := m.teachersByWeek[key]; ok {
		return t, nil
	}
	return nil, fmt.Errorf("teacher schedule not found")
}

func (m *mockNavTeacherSvc) GetAvailableWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error) {
	key := week.Format("2006-01-02")
	if nav, ok := m.weeksNav[key]; ok {
		return nav, nil
	}
	return entities.WeekNavigation{}, nil
}

func (m *mockNavTeacherSvc) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (m *mockNavTeacherSvc) GetAllWeeks(ctx context.Context, name string) ([]time.Time, error) {
	return nil, nil
}

func (m *mockNavTeacherSvc) GetTeacherArchiveSchedule(ctx context.Context, name string, targetDate time.Time) (*entities.Teacher, error) {
	return nil, nil
}

func (m *mockNavTeacherSvc) ValidateTeacher(ctx context.Context, name string) (string, []string, bool) {
	return name, nil, true
}

func (m *mockNavTeacherSvc) FindSimilarTeachers(ctx context.Context, name string, limit int) ([]string, error) {
	return nil, nil
}

func (m *mockNavTeacherSvc) GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error) {
	return nil, nil
}

func makeWeekDays(prefix string) []*entities.Day {
	dayNames := []string{"Понедельник", "Вторник", "Среда", "Четверг", "Пятница", "Суббота"}
	days := make([]*entities.Day, len(dayNames))
	for i, d := range dayNames {
		days[i] = &entities.Day{
			Name: d,
			Subjects: []*entities.Subject{
				{
					Pairs: []*entities.Pair{
						{Name: fmt.Sprintf("%s %s урок", prefix, d)},
					},
				},
			},
		}
	}
	return days
}

func TestCrossWeekGroupDayNavigation(t *testing.T) {
	week1Monday := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	week2Monday := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	grpWeek1 := &entities.Group{
		ID:   101,
		Week: week1Monday,
		Days: makeWeekDays("W1"),
	}
	grpWeek2 := &entities.Group{
		ID:   101,
		Week: week2Monday,
		Days: makeWeekDays("W2"),
	}

	schedSvc := &mockNavScheduleSvc{
		groupsByWeek: map[string]*entities.Group{
			"101:2026-10-12": grpWeek1,
			"101:2026-10-19": grpWeek2,
		},
		latestGroup: grpWeek2,
		weeksNav: map[string]entities.WeekNavigation{
			"2026-10-12": {Current: week1Monday, Next: week2Monday},
			"2026-10-19": {Prev: week1Monday, Current: week2Monday},
		},
	}

	userRepo := newMockUserRepo()
	sm := state.NewMemoryManager()
	cbHandler := handlers.CallbacksHandler(slog.New(slog.DiscardHandler), schedSvc, nil, nil, userRepo, sm)

	t.Run("transition from saturday week1 to monday week2", func(t *testing.T) {
		// Кнопка nextday с субботы W1 указывает на W2 19.10.2026, день 0
		ctx := &mockContext{
			chatID:   123,
			callback: &tele.Callback{Data: "\f" + keyboard.NextDay + "|101:19.10.2026:0:0"},
		}

		err := cbHandler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.editedWhat, "W2")
		assert.Contains(t, ctx.editedWhat, "Понедельник")

		// Проверяем клавиатуру: на понедельнике W2 кнопка prevday должна вести на субботу (-1) W1!
		rm, ok := ctx.editedOpts[1].(*tele.ReplyMarkup)
		require.True(t, ok)
		require.NotEmpty(t, rm.InlineKeyboard)

		firstRow := rm.InlineKeyboard[0]
		require.NotEmpty(t, firstRow)

		var prevBtnData, nextBtnData string
		for _, b := range firstRow {
			if b.Unique == keyboard.PrevDay {
				prevBtnData = b.Data
			}
			if b.Unique == keyboard.NextDay {
				nextBtnData = b.Data
			}
		}

		assert.Equal(t, "101:12.10.2026:-1:0", prevBtnData, "на понедельнике W2 кнопка назад должна вести на субботу прошлой недели W1")
		assert.Equal(t, "101:19.10.2026:1:0", nextBtnData, "на понедельнике W2 кнопка вперед должна вести на вторник этой же недели W2")
	})

	t.Run("transition from monday week2 to saturday week1 with -1 dayIdx", func(t *testing.T) {
		ctx := &mockContext{
			chatID:   123,
			callback: &tele.Callback{Data: "\f" + keyboard.PrevDay + "|101:12.10.2026:-1:0"},
		}

		err := cbHandler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.editedWhat, "W1")
		assert.Contains(t, ctx.editedWhat, "Суббота")

		// На субботе W1 кнопка nextday должна вести на понедельник (0) недели W2!
		rm, ok := ctx.editedOpts[1].(*tele.ReplyMarkup)
		require.True(t, ok)
		firstRow := rm.InlineKeyboard[0]

		var prevBtnData, nextBtnData string
		for _, b := range firstRow {
			if b.Unique == keyboard.PrevDay {
				prevBtnData = b.Data
			}
			if b.Unique == keyboard.NextDay {
				nextBtnData = b.Data
			}
		}

		assert.Equal(t, "101:12.10.2026:4:0", prevBtnData, "на субботе W1 кнопка назад должна вести на пятницу W1")
		assert.Equal(t, "101:19.10.2026:0:0", nextBtnData, "на субботе W1 кнопка вперед должна вести на понедельник W2")
	})

	t.Run("iterate inside week1 maintains week1 date in day_sg and nav buttons", func(t *testing.T) {
		ctx := &mockContext{
			chatID:   123,
			callback: &tele.Callback{Data: "\f" + keyboard.PrevDay + "|101:12.10.2026:4:0"},
		}

		err := cbHandler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.editedWhat, "W1")
		assert.Contains(t, ctx.editedWhat, "Пятница")

		rm, ok := ctx.editedOpts[1].(*tele.ReplyMarkup)
		require.True(t, ok)

		// Проверяем кнопки подгрупп: дата недели W1 должна сохраняться!
		sgRow := rm.InlineKeyboard[1]
		assert.Contains(t, sgRow[0].Data, "101:12.10.2026:4:1")
		assert.Contains(t, sgRow[1].Data, "101:12.10.2026:4:0")
		assert.Contains(t, sgRow[2].Data, "101:12.10.2026:4:2")

		// Переключаем подгруппу на 2
		ctxSg := &mockContext{
			chatID:   123,
			callback: &tele.Callback{Data: "\f" + keyboard.DaySubgroup + "|101:12.10.2026:4:2"},
		}
		err = cbHandler(ctxSg)
		require.NoError(t, err)
		assert.Contains(t, ctxSg.editedWhat, "W1")
		assert.Contains(t, ctxSg.editedWhat, "Пятница")
	})

	t.Run("legacy callback format without date falls back to latest schedule", func(t *testing.T) {
		ctx := &mockContext{
			chatID:   123,
			callback: &tele.Callback{Data: "\f" + keyboard.NextDay + "|101:1:0"},
		}

		err := cbHandler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.editedWhat, "W2")
		assert.Contains(t, ctx.editedWhat, "Вторник")
	})
}

func TestCrossWeekTeacherDayNavigation(t *testing.T) {
	week1Monday := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	week2Monday := time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)

	tWeek1 := &entities.Teacher{
		Name: "Иванов И. И.",
		Week: week1Monday,
		Days: makeWeekDays("TW1"),
	}
	tWeek2 := &entities.Teacher{
		Name: "Иванов И. И.",
		Week: week2Monday,
		Days: makeWeekDays("TW2"),
	}

	tSvc := &mockNavTeacherSvc{
		teachersByWeek: map[string]*entities.Teacher{
			"Иванов И. И.:2026-10-12": tWeek1,
			"Иванов И. И.:2026-10-19": tWeek2,
		},
		latestTeacher: tWeek2,
		weeksNav: map[string]entities.WeekNavigation{
			"2026-10-12": {Current: week1Monday, Next: week2Monday},
			"2026-10-19": {Prev: week1Monday, Current: week2Monday},
		},
	}

	userRepo := newMockUserRepo()
	sm := state.NewMemoryManager()
	cbHandler := handlers.CallbacksHandler(slog.New(slog.DiscardHandler), nil, tSvc, nil, userRepo, sm)

	t.Run("transition from saturday week1 to monday week2 for teacher", func(t *testing.T) {
		ctx := &mockContext{
			chatID:   123,
			callback: &tele.Callback{Data: "\f" + keyboard.TeacherNextDay + "|Иванов И. И.:19.10.2026:0"},
		}

		err := cbHandler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.editedWhat, "TW2")
		assert.Contains(t, ctx.editedWhat, "Понедельник")

		rm, ok := ctx.editedOpts[1].(*tele.ReplyMarkup)
		require.True(t, ok)
		firstRow := rm.InlineKeyboard[0]

		var prevBtnData, nextBtnData string
		for _, b := range firstRow {
			if b.Unique == keyboard.TeacherPrevDay {
				prevBtnData = b.Data
			}
			if b.Unique == keyboard.TeacherNextDay {
				nextBtnData = b.Data
			}
		}

		assert.Equal(t, "Иванов И. И.:12.10.2026:-1", prevBtnData)
		assert.Equal(t, "Иванов И. И.:19.10.2026:1", nextBtnData)
	})

	t.Run("transition from monday week2 to saturday week1 for teacher", func(t *testing.T) {
		ctx := &mockContext{
			chatID:   123,
			callback: &tele.Callback{Data: "\f" + keyboard.TeacherPrevDay + "|Иванов И. И.:12.10.2026:-1"},
		}

		err := cbHandler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.editedWhat, "TW1")
		assert.Contains(t, ctx.editedWhat, "Суббота")
	})

	t.Run("legacy teacher day callback without date", func(t *testing.T) {
		ctx := &mockContext{
			chatID:   123,
			callback: &tele.Callback{Data: "\f" + keyboard.TeacherNextDay + "|Иванов И. И.:1"},
		}

		err := cbHandler(ctx)
		require.NoError(t, err)
		assert.Contains(t, ctx.editedWhat, "TW2")
		assert.Contains(t, ctx.editedWhat, "Вторник")
	})
}
