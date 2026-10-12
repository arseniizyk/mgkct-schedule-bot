package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
)

type ScheduleRepository interface {
	Save(ctx context.Context, week time.Time, schedule *entities.Schedule) error
	GetByWeek(ctx context.Context, week time.Time) (*entities.Schedule, error)
	GetLatest(ctx context.Context) (*entities.Schedule, error)
	GetAll(ctx context.Context) (map[time.Time]*entities.Schedule, error)
	GetWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error)
}

type ScheduleService struct {
	log          *slog.Logger
	scheduleRepo ScheduleRepository

	mu    sync.RWMutex
	cache *entities.Schedule
}

func NewScheduleService(log *slog.Logger, scheduleRepo ScheduleRepository) *ScheduleService {
	return &ScheduleService{
		log:          log,
		scheduleRepo: scheduleRepo,
	}
}

func (s *ScheduleService) GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error) {
	const op = "service.ScheduleService.GetGroupLatestSchedule"
	log := s.log.With("operation", op, "group_id", groupID)

	s.mu.RLock()
	if s.cache != nil {
		group, ok := s.cache.Groups[groupID]
		s.mu.RUnlock()
		if ok {
			return group, nil
		}
	} else {
		s.mu.RUnlock()
	}

	sch, err := s.scheduleRepo.GetLatest(ctx)
	if err != nil {
		if errors.Is(err, domainerr.ErrScheduleNotFound) {
			log.WarnContext(ctx, "schedule not found in repository")
			return nil, err
		}
		log.ErrorContext(ctx, "failed get by latest", "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	s.SetCache(sch)

	group, ok := sch.Groups[groupID]
	if !ok {
		log.WarnContext(ctx, "group not found in latest schedule")
		return nil, domainerr.ErrGroupNotFound
	}

	return group, nil
}

func (s *ScheduleService) GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
	const op = "service.ScheduleService.GetGroupScheduleByWeek"
	log := s.log.With("operation", op, "group_id", groupID, "week", week)

	sch, err := s.scheduleRepo.GetByWeek(ctx, week)
	if err != nil {
		if errors.Is(err, domainerr.ErrWeekNotFound) {
			log.WarnContext(ctx, "week not found")
			return nil, err
		}
		log.ErrorContext(ctx, "failed get by week", "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	group, ok := sch.Groups[groupID]
	if !ok {
		log.WarnContext(ctx, "group not found for week")
		return nil, domainerr.ErrGroupNotFound
	}

	return group, nil
}

func (s *ScheduleService) GetAvailableWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error) {
	return s.scheduleRepo.GetWeeks(ctx, week)
}

func (s *ScheduleService) SetCache(sch *entities.Schedule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = sch
}

func (s *ScheduleService) GetAllGroupIDs(ctx context.Context) ([]int32, error) {
	const op = "service.ScheduleService.GetAllGroupIDs"

	s.mu.RLock()
	if s.cache != nil && len(s.cache.Groups) > 0 {
		ids := make([]int32, 0, len(s.cache.Groups))
		for id := range s.cache.Groups {
			ids = append(ids, id)
		}
		s.mu.RUnlock()
		slices.Sort(ids)
		return ids, nil
	}
	s.mu.RUnlock()

	sch, err := s.scheduleRepo.GetLatest(ctx)
	if err != nil {
		if errors.Is(err, domainerr.ErrScheduleNotFound) {
			return []int32{}, nil
		}
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	s.SetCache(sch)
	ids := make([]int32, 0, len(sch.Groups))
	for id := range sch.Groups {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids, nil
}
