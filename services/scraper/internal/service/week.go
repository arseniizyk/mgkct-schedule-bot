package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/repository"
)

type WeekService struct {
	log      *slog.Logger
	weekRepo WeekRepository
}

type WeekRepository interface {
	GetWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error)
}

func NewWeekService(log *slog.Logger, weekRepoitory WeekRepository) *WeekService {
	return &WeekService{
		log:      log,
		weekRepo: weekRepoitory,
	}
}

func (ws *WeekService) GetAvailableWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error) {
	log := ws.log.With(
		"operation", "service.week.WeekService.GetAvailableWeeks",
		"week", week.String(),
	)
	weeks, err := ws.weekRepo.GetWeeks(ctx, week)
	if err != nil {
		if !errors.Is(err, repository.ErrNoAvailableWeeks) {
			log.Error("failed get available weeks", "err", err)
			return entities.WeekNavigation{}, err
		}
		return entities.WeekNavigation{}, nil
	}

	return weeks, nil
}
