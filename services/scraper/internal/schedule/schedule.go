package schedule

import (
	"context"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
)

type ScheduleUsecase interface {
	GetLatest() (*models.Schedule, error)
	SaveToCache(sch *models.Schedule)
	Save(ctx context.Context, week time.Time, sch *models.Schedule) error
}
