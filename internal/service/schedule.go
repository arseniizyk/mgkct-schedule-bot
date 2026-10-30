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
	GetAllWeeks(ctx context.Context) ([]time.Time, error)
	GetWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error)
}

type ScheduleService struct {
	log          *slog.Logger
	scheduleRepo ScheduleRepository

	mu        sync.RWMutex
	cache     *entities.Schedule
	allGroups []int32
	nowFunc   func() time.Time
}

func NewScheduleService(log *slog.Logger, scheduleRepo ScheduleRepository) *ScheduleService {
	return &ScheduleService{
		log:          log,
		scheduleRepo: scheduleRepo,
	}
}

func (s *ScheduleService) now() time.Time {
	if s.nowFunc != nil {
		return s.nowFunc()
	}
	return time.Now()
}

func (s *ScheduleService) SetNowFunc(fn func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nowFunc = fn
}

func (s *ScheduleService) GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error) {
	const op = "service.ScheduleService.GetGroupLatestSchedule"
	log := s.log.With("operation", op, "group_id", groupID)

	targetWeek := EffectiveScheduleWeek(s.now())

	s.mu.RLock()
	if s.cache != nil {
		group, ok := s.cache.Groups[groupID]
		if ok && CurrentWeekMonday(group.Week).Equal(CurrentWeekMonday(targetWeek)) {
			s.mu.RUnlock()
			return group, nil
		}
	}
	s.mu.RUnlock()

	sch, err := s.scheduleRepo.GetByWeek(ctx, targetWeek)
	if err != nil || sch == nil {
		// Fallback к последней сохраненной неделе, если targetWeek еще не существует
		sch, err = s.scheduleRepo.GetLatest(ctx)
		if err != nil {
			if errors.Is(err, domainerr.ErrScheduleNotFound) {
				log.WarnContext(ctx, "schedule not found in repository")
				return nil, err
			}
			log.ErrorContext(ctx, "failed get by latest", "error", err)
			return nil, fmt.Errorf("%s: %w", op, err)
		}
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
	s.allGroups = nil
}

func (s *ScheduleService) GetAllGroupIDs(ctx context.Context) ([]int32, error) {
	const op = "service.ScheduleService.GetAllGroupIDs"

	s.mu.RLock()
	if len(s.allGroups) > 0 {
		defer s.mu.RUnlock()
		cp := make([]int32, len(s.allGroups))
		copy(cp, s.allGroups)
		return cp, nil
	}
	s.mu.RUnlock()

	schedules, err := s.scheduleRepo.GetAll(ctx)
	if err != nil && !errors.Is(err, domainerr.ErrScheduleNotFound) {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	groupMap := make(map[int32]struct{})
	for _, sch := range schedules {
		for id := range sch.Groups {
			groupMap[id] = struct{}{}
		}
	}

	if len(groupMap) == 0 {
		s.mu.RLock()
		if s.cache != nil {
			for id := range s.cache.Groups {
				groupMap[id] = struct{}{}
			}
		}
		s.mu.RUnlock()

		if len(groupMap) == 0 {
			sch, err := s.scheduleRepo.GetLatest(ctx)
			if err != nil {
				if errors.Is(err, domainerr.ErrScheduleNotFound) {
					return []int32{}, nil
				}
				return nil, fmt.Errorf("%s: %w", op, err)
			}
			s.SetCache(sch)
			for id := range sch.Groups {
				groupMap[id] = struct{}{}
			}
		}
	}

	ids := make([]int32, 0, len(groupMap))
	for id := range groupMap {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	s.mu.Lock()
	s.allGroups = ids
	s.mu.Unlock()

	cp := make([]int32, len(ids))
	copy(cp, ids)
	return cp, nil
}

func (s *ScheduleService) HasGroup(ctx context.Context, groupID int32) (bool, error) {
	const op = "service.ScheduleService.HasGroup"

	ids, err := s.GetAllGroupIDs(ctx)
	if err != nil {
		return false, fmt.Errorf("%s: %w", op, err)
	}
	return slices.Contains(ids, groupID), nil
}

func (s *ScheduleService) GetAllWeeks(ctx context.Context) ([]time.Time, error) {
	return s.scheduleRepo.GetAllWeeks(ctx)
}

func (s *ScheduleService) GetArchiveSchedule(ctx context.Context, groupID int32, targetDate time.Time) (*entities.Group, error) {
	const op = "service.ScheduleService.GetArchiveSchedule"

	targetDate = time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, time.UTC)
	weekday := (int(targetDate.Weekday()) + 6) % 7 // Пн=0..Вс=6
	monday := targetDate.AddDate(0, 0, -weekday)

	grp, err := s.GetGroupScheduleByWeek(ctx, groupID, monday)
	if err == nil {
		return grp, nil
	}

	sch, err := s.scheduleRepo.GetByWeek(ctx, targetDate)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	group, ok := sch.Groups[groupID]
	if !ok {
		return nil, domainerr.ErrGroupNotFound
	}

	diff := targetDate.Sub(group.Week)
	if diff < 0 || diff > 7*24*time.Hour {
		return nil, domainerr.ErrWeekNotFound
	}

	return group, nil
}
