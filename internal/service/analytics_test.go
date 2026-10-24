package service_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/service"
)

func TestAnalyticsService_GetCabinetSchedule(t *testing.T) {
	ctx := context.Background()

	now := time.Now()
	sch := &entities.Schedule{
		Groups: map[int32]*entities.Group{
			101: {
				ID:   101,
				Week: now,
				Days: []*entities.Day{
					{
						Name: "Понедельник",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "ТОЭ", Type: "Лек", Group: "Иванов И. И.", Class: "301"}}},
							{Pairs: []*entities.Pair{{Name: "Математика", Type: "ПЗ", Group: "Петров П. П.", Class: "3-207"}}},
						},
					},
				},
			},
			102: {
				ID:   102,
				Week: now,
				Days: []*entities.Day{
					{
						Name: "Понедельник",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "Физика", Type: "Лек", Group: "Сидоров С. С.", Class: "301"}}},
						},
					},
				},
			},
		},
	}

	schedRepo := &mockScheduleRepo{schedules: map[time.Time]*entities.Schedule{now: sch}}
	svc := service.NewAnalyticsService(slog.Default(), schedRepo, nil, nil)

	// 1. Поиск кабинета "301"
	cabSched, candidates, err := svc.GetCabinetSchedule(ctx, "301", time.Time{})
	require.NoError(t, err)
	assert.Empty(t, candidates)
	require.NotNil(t, cabSched)
	assert.Equal(t, "301", cabSched.Cabinet)
	require.Len(t, cabSched.Days, 1)
	assert.Len(t, cabSched.Days[0].Bookings, 2)

	// 2. Поиск кабинета без дефиса "3207" (нормализованный поиск для "3-207")
	cabSched, candidates, err = svc.GetCabinetSchedule(ctx, "3207", time.Time{})
	require.NoError(t, err)
	assert.Empty(t, candidates)
	require.NotNil(t, cabSched)
	assert.Equal(t, "3-207", cabSched.Cabinet)

	// 3. Список всех кабинетов
	cabinets, err := svc.GetAllCabinets(ctx)
	require.NoError(t, err)
	assert.Equal(t, []string{"3-207", "301"}, cabinets)
}

func TestAnalyticsService_GetEndings(t *testing.T) {
	ctx := context.Background()

	now := time.Now()
	sch := &entities.Schedule{
		Groups: map[int32]*entities.Group{
			101: {
				ID:   101,
				Week: now,
				Days: []*entities.Day{
					{
						Name: "Понедельник",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "Пара 1"}}},
							{Pairs: []*entities.Pair{{Name: "Пара 2"}}},
							{Pairs: []*entities.Pair{{Name: "Пара 3"}}},
						},
					},
				},
			},
			102: {
				ID:   102,
				Week: now,
				Days: []*entities.Day{
					{
						Name: "Понедельник",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "Пара 1"}}},
							{Pairs: []*entities.Pair{{Name: "Пара 2"}}},
						},
					},
				},
			},
			103: {
				ID:   103,
				Week: now,
				Days: []*entities.Day{
					{
						Name:     "Понедельник",
						Subjects: []*entities.Subject{}, // выходной
					},
				},
			},
		},
	}

	schedRepo := &mockScheduleRepo{schedules: map[time.Time]*entities.Schedule{now: sch}}
	svc := service.NewAnalyticsService(slog.Default(), schedRepo, nil, nil)

	stat, err := svc.GetEndings(ctx, 0, time.Time{})
	require.NoError(t, err)
	require.NotNil(t, stat)

	// Группа 101 заканчивает после 3 пары
	assert.Equal(t, []entities.GroupEnding{{GroupID: 101}}, stat.Pairs[3])
	// Группа 102 заканчивает после 2 пары
	assert.Equal(t, []entities.GroupEnding{{GroupID: 102}}, stat.Pairs[2])
	// Группа 103 выходной
	assert.Equal(t, []entities.GroupEnding{{GroupID: 103}}, stat.DayOff)
}

func TestAnalyticsService_GetEndings_SaturdayCurrentWeekVsNextWeek(t *testing.T) {
	ctx := context.Background()
	minskLoc := time.FixedZone("MSK", 3*3600)

	currentMonday := time.Date(2026, 10, 19, 0, 0, 0, 0, minskLoc)
	nextMonday := time.Date(2026, 10, 26, 0, 0, 0, 0, minskLoc)

	currentSchedule := &entities.Schedule{
		Groups: map[int32]*entities.Group{
			101: {
				ID:   101,
				Week: currentMonday,
				Days: []*entities.Day{
					{Name: "Понедельник", Subjects: []*entities.Subject{{Pairs: []*entities.Pair{{Name: "Пара 1"}}}}},
					{Name: "Вторник"},
					{Name: "Среда"},
					{Name: "Четверг"},
					{Name: "Пятница"},
					{
						Name: "Суббота",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "Пара 1"}}},
							{Pairs: []*entities.Pair{{Name: "Пара 2"}}}, // 2 пары в субботу на этой неделе
						},
					},
				},
			},
		},
	}

	nextSchedule := &entities.Schedule{
		Groups: map[int32]*entities.Group{
			101: {
				ID:   101,
				Week: nextMonday,
				Days: []*entities.Day{
					{
						Name: "Понедельник",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "Новая неделя пара 1"}}},
							{Pairs: []*entities.Pair{{Name: "Новая неделя пара 2"}}},
							{Pairs: []*entities.Pair{{Name: "Новая неделя пара 3"}}},
						},
					},
					{Name: "Вторник"},
					{Name: "Среда"},
					{Name: "Четверг"},
					{Name: "Пятница"},
					{
						Name: "Суббота",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "Пара 1"}}},
							{Pairs: []*entities.Pair{{Name: "Пара 2"}}},
							{Pairs: []*entities.Pair{{Name: "Пара 3"}}},
							{Pairs: []*entities.Pair{{Name: "Пара 4"}}}, // 4 пары в субботу на следующей неделе
						},
					},
				},
			},
		},
	}

	schedRepo := &mockScheduleRepo{
		schedules: map[time.Time]*entities.Schedule{
			currentMonday: currentSchedule,
			nextMonday:    nextSchedule,
		},
	}
	svc := service.NewAnalyticsService(slog.Default(), schedRepo, nil, nil)

	// 1. Суббота 11:00 (Минск) — день еще идет
	saturdayMorning := time.Date(2026, 10, 24, 11, 0, 0, 0, minskLoc)
	svc.SetNowFunc(func() time.Time { return saturdayMorning })

	dayIdx := svc.EffectiveEndingsDayIndex()
	assert.Equal(t, 5, dayIdx, "в субботу утром dayIdx должен быть 5 (суббота)")

	stat, err := svc.GetEndings(ctx, dayIdx, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, "Суббота", stat.DayName)
	assert.Equal(t, []entities.GroupEnding{{GroupID: 101}}, stat.Pairs[2], "в субботу утром должно показываться расписание текущей недели (2 пары), а не следующей (4 пары)")
	assert.Empty(t, stat.Pairs[4])

	// 2. Суббота 20:00 (Минск) — вечер, учебная неделя кончилась
	saturdayEvening := time.Date(2026, 10, 24, 20, 0, 0, 0, minskLoc)
	svc.SetNowFunc(func() time.Time { return saturdayEvening })

	dayIdx = svc.EffectiveEndingsDayIndex()
	assert.Equal(t, 0, dayIdx, "в субботу вечером dayIdx должен переключиться на 0 (понедельник)")

	stat, err = svc.GetEndings(ctx, dayIdx, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, "Понедельник", stat.DayName)
	assert.Equal(t, []entities.GroupEnding{{GroupID: 101}}, stat.Pairs[3], "в субботу вечером должен показываться понедельник следующей недели (3 пары)")

	// 3. Воскресенье 12:00 (Минск)
	sundayNoon := time.Date(2026, 10, 25, 12, 0, 0, 0, minskLoc)
	svc.SetNowFunc(func() time.Time { return sundayNoon })

	dayIdx = svc.EffectiveEndingsDayIndex()
	assert.Equal(t, 0, dayIdx, "в воскресенье dayIdx должен быть 0 (понедельник)")

	stat, err = svc.GetEndings(ctx, dayIdx, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, "Понедельник", stat.DayName)
	assert.Equal(t, []entities.GroupEnding{{GroupID: 101}}, stat.Pairs[3], "в воскресенье должен показываться понедельник следующей недели")
}

func TestAnalyticsService_GetEndings_SubgroupsSplit(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	// Группа 100: 1-я подгруппа заканчивает к 5 паре, 2-я подгруппа заканчивает к 3 паре.
	// Группа 101: без подгрупп, заканчивает к 4 паре.
	// Группа 102: подгруппы есть, но обе заканчивают к 3 паре.
	sch := &entities.Schedule{
		Groups: map[int32]*entities.Group{
			100: {
				ID:   100,
				Week: now,
				Days: []*entities.Day{
					{
						Name: "Понедельник",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "ТОЭ", Type: "Лек"}}},                // 1 пара: общая
							{Pairs: []*entities.Pair{{Name: "ЭлИз", Type: "Лек"}}},               // 2 пара: общая
							{Pairs: []*entities.Pair{{Name: "2. Основы инж гр", Type: "ЛР"}}},    // 3 пара: 2-я подгруппа (для нее последняя!)
							{Pairs: []*entities.Pair{{Name: "1. Программирование", Type: "ЛР"}}}, // 4 пара: 1-я подгруппа
							{Pairs: []*entities.Pair{{Name: "1. Программирование", Type: "ЛР"}}}, // 5 пара: 1-я подгруппа (для нее последняя!)
						},
					},
				},
			},
			101: {
				ID:   101,
				Week: now,
				Days: []*entities.Day{
					{
						Name: "Понедельник",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "История"}}},
							{Pairs: []*entities.Pair{{Name: "Математика"}}},
							{Pairs: []*entities.Pair{{Name: "Физика"}}},
							{Pairs: []*entities.Pair{{Name: "Английский"}}}, // 4 пара
						},
					},
				},
			},
			102: {
				ID:   102,
				Week: now,
				Days: []*entities.Day{
					{
						Name: "Понедельник",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "История"}}},
							{Pairs: []*entities.Pair{{Name: "Математика"}}},
							{Pairs: []*entities.Pair{
								{Name: "1. Ин Яз"},
								{Name: "2. Ин Яз"},
							}}, // 3 пара: обе подгруппы заканчивают в одно время
						},
					},
				},
			},
		},
	}

	schedRepo := &mockScheduleRepo{schedules: map[time.Time]*entities.Schedule{now: sch}}
	svc := service.NewAnalyticsService(slog.Default(), schedRepo, nil, nil)

	stat, err := svc.GetEndings(ctx, 0, now)
	require.NoError(t, err)
	require.NotNil(t, stat)

	// 1. Проверяем 3-ю пару:
	// Группа 100 должна быть представлена только 2-й подгруппой (Subgroup = 2)
	// Группа 102 не разбита (Subgroup = 0)
	expectedPair3 := []entities.GroupEnding{
		{GroupID: 100, Subgroup: 2},
		{GroupID: 102, Subgroup: 0},
	}
	assert.Equal(t, expectedPair3, stat.Pairs[3])

	// 2. Проверяем 4-ю пару:
	// Группа 101 не разбита (Subgroup = 0)
	expectedPair4 := []entities.GroupEnding{
		{GroupID: 101, Subgroup: 0},
	}
	assert.Equal(t, expectedPair4, stat.Pairs[4])

	// 3. Проверяем 5-ю пару:
	// Группа 100 должна быть представлена только 1-й подгруппой (Subgroup = 1)
	expectedPair5 := []entities.GroupEnding{
		{GroupID: 100, Subgroup: 1},
	}
	assert.Equal(t, expectedPair5, stat.Pairs[5])

	// Проверяем форматирование
	assert.Equal(t, "100 (2 п/г)", stat.Pairs[3][0].String())
	assert.Equal(t, "102", stat.Pairs[3][1].String())
	assert.Equal(t, "100 (1 п/г)", stat.Pairs[5][0].String())
}

func TestAnalyticsService_GetGroupStats(t *testing.T) {
	ctx := context.Background()

	now := time.Now()
	sch := &entities.Schedule{
		Groups: map[int32]*entities.Group{
			101: {
				ID:   101,
				Week: now,
				Days: []*entities.Day{
					{
						Name: "Пн",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "ТОЭ", Type: "Лек"}}},
							{Pairs: []*entities.Pair{{Name: "ТОЭ", Type: "ЛР"}}},
							{Pairs: []*entities.Pair{{Name: "Математика", Type: "ПЗ"}}},
						},
					},
				},
			},
		},
	}

	schedRepo := &mockScheduleRepo{schedules: map[time.Time]*entities.Schedule{now: sch}}
	svc := service.NewAnalyticsService(slog.Default(), schedRepo, nil, nil)

	stats, total, err := svc.GetGroupStats(ctx, 101, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, 3, total)
	require.Len(t, stats, 2)

	// ТОЭ на 1-м месте (2 пары)
	assert.Equal(t, "ТОЭ", stats[0].Subject)
	assert.Equal(t, 2, stats[0].Count)
	assert.Equal(t, 1, stats[0].Types["Лек"])
	assert.Equal(t, 1, stats[0].Types["ЛР"])

	// Математика на 2-м месте (1 пара)
	assert.Equal(t, "Математика", stats[1].Subject)
	assert.Equal(t, 1, stats[1].Count)
}
