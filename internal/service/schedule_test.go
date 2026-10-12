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
