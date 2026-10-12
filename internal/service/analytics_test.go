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

	stat, err := svc.GetEndings(ctx, 0)
	require.NoError(t, err)
	require.NotNil(t, stat)

	// Группа 101 заканчивает после 3 пары
	assert.Equal(t, []int32{101}, stat.Pairs[3])
	// Группа 102 заканчивает после 2 пары
	assert.Equal(t, []int32{102}, stat.Pairs[2])
	// Группа 103 выходной
	assert.Equal(t, []int32{103}, stat.DayOff)
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
