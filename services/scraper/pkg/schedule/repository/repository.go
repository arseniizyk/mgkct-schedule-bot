package repository

import (
	"context"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
)

type ScheduleRepository interface {
	Save(ctx context.Context, week time.Time, schedule *models.Schedule) error
	GetByWeek(ctx context.Context, week time.Time) (*models.Schedule, error)
	GetLatest(ctx context.Context) (*models.Schedule, error)
}
