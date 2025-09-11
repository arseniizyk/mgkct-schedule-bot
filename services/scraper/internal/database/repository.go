package database

import (
	"context"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
)

type DatabaseRepository interface {
	SaveSchedule(ctx context.Context, week time.Time, schedule *models.Schedule) error
	GetSchedule(ctx context.Context, week time.Time) (*models.Schedule, error)
	GetLatestSchedule(ctx context.Context) (*models.Schedule, error)
	Ping(ctx context.Context) error
	Close()
}
