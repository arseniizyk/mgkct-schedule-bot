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

func TestScheduleService_Archive(t *testing.T) {
	ctx := context.Background()

	week, _ := time.Parse("2006-01-02", "2026-10-12")
	schedRepo := &mockScheduleRepo{schedules: make(map[time.Time]*entities.Schedule)}
	_ = schedRepo.Save(ctx, week, &entities.Schedule{
		Groups: map[int32]*entities.Group{
			101: {ID: 101, Week: week},
		},
	})

	svc := service.NewScheduleService(slog.Default(), schedRepo)

	// GetAllWeeks
	weeks, err := svc.GetAllWeeks(ctx)
	require.NoError(t, err)
	assert.Len(t, weeks, 1)

	// GetArchiveSchedule exact monday
	targetMonday, _ := time.Parse("2006-01-02", "2026-10-12")
	grp, err := svc.GetArchiveSchedule(ctx, 101, targetMonday)
	require.NoError(t, err)
	assert.Equal(t, int32(101), grp.ID)

	// GetArchiveSchedule midweek date (Friday 16.10)
	midweek, _ := time.Parse("2006-01-02", "2026-10-16")
	grp, err = svc.GetArchiveSchedule(ctx, 101, midweek)
	require.NoError(t, err)
	assert.Equal(t, int32(101), grp.ID)

	// Non-existent group
	_, err = svc.GetArchiveSchedule(ctx, 999, targetMonday)
	require.Error(t, err)
}

func TestScheduleService_GetGroupLatestSchedule_SaturdayCurrentWeekVsNextWeek(t *testing.T) {
	ctx := context.Background()
	minskLoc := time.FixedZone("MSK", 3*3600)

	currentMonday := time.Date(2026, 10, 19, 0, 0, 0, 0, minskLoc)
	nextMonday := currentMonday.AddDate(0, 0, 7) // 2026-10-26

	schedRepo := &mockScheduleRepo{schedules: make(map[time.Time]*entities.Schedule)}
	_ = schedRepo.Save(ctx, currentMonday, &entities.Schedule{
		Groups: map[int32]*entities.Group{
			101: {ID: 101, Week: currentMonday},
		},
	})
	_ = schedRepo.Save(ctx, nextMonday, &entities.Schedule{
		Groups: map[int32]*entities.Group{
			101: {ID: 101, Week: nextMonday},
		},
	})

	svc := service.NewScheduleService(slog.Default(), schedRepo)

	// 1. Суббота 10:00 (утро) — еще идет текущая неделя
	saturdayMorning := time.Date(2026, 10, 24, 10, 0, 0, 0, minskLoc)
	svc.SetNowFunc(func() time.Time { return saturdayMorning })

	grp, err := svc.GetGroupLatestSchedule(ctx, 101)
	require.NoError(t, err)
	assert.Equal(t, currentMonday, grp.Week, "в субботу утром должно возвращаться расписание текущей недели, а не следующей")

	// 2. Суббота 20:00 (вечер) — учебная неделя кончилась, показываем следующую
	saturdayEvening := time.Date(2026, 10, 24, 20, 0, 0, 0, minskLoc)
	svc.SetNowFunc(func() time.Time { return saturdayEvening })

	grp, err = svc.GetGroupLatestSchedule(ctx, 101)
	require.NoError(t, err)
	assert.Equal(t, nextMonday, grp.Week, "в субботу вечером должно возвращаться расписание следующей недели")

	// 3. Воскресенье 12:00
	sundayNoon := time.Date(2026, 10, 25, 12, 0, 0, 0, minskLoc)
	svc.SetNowFunc(func() time.Time { return sundayNoon })

	grp, err = svc.GetGroupLatestSchedule(ctx, 101)
	require.NoError(t, err)
	assert.Equal(t, nextMonday, grp.Week, "в воскресенье должно возвращаться расписание следующей недели")
}
