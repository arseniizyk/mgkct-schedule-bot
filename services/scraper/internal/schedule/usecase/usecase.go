package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/repository"
)

type ScheduleUsecase struct {
	repo  repository.ScheduleRepository
	cache *models.Schedule
}

func New(repo repository.ScheduleRepository) *ScheduleUsecase {
	return &ScheduleUsecase{
		repo:  repo,
		cache: nil,
	}
}

func (s *ScheduleUsecase) GetLatest() (*models.Schedule, error) {
	if s.cache != nil {
		return s.cache, nil
	}

	sch, err := s.repo.GetLatest(context.Background())
	if err != nil {
		return nil, fmt.Errorf("usecase: get latest schedule: %w", err)
	}

	s.cache = sch
	return sch, nil
}

func (s *ScheduleUsecase) SaveToCache(sch *models.Schedule) {
	s.cache = sch
}

func (s *ScheduleUsecase) Save(ctx context.Context, week time.Time, sch *models.Schedule) error {
	return s.repo.Save(ctx, week, sch)
}
