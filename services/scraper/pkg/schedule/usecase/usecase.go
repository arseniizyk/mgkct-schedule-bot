package usecase

import (
	"context"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/database"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
)

type ScheduleUsecase struct {
	db    database.DatabaseRepository
	cache *models.Schedule
}

func NewScheduleUsecase(db database.DatabaseRepository) *ScheduleUsecase {
	return &ScheduleUsecase{db: db}
}

func (s *ScheduleUsecase) GetLatest() (*models.Schedule, error) {
	if s.cache != nil {
		return s.cache, nil
	}

	sch, err := s.db.GetLatestSchedule(context.Background())
	if err != nil {
		return nil, err
	}

	s.cache = sch
	return sch, nil
}

func (s *ScheduleUsecase) SaveToCache(sch *models.Schedule) {
	s.cache = sch
}
