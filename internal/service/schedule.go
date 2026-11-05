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

const scheduleCacheTTL = 5 * time.Minute

type scheduleCacheItem struct {
	schedule *entities.Schedule
	cachedAt time.Time
}

type ScheduleService struct {
	log          *slog.Logger
	scheduleRepo ScheduleRepository

	mu               sync.RWMutex
	cache            *entities.Schedule
	cachedTargetWeek time.Time
	cacheAt          time.Time
	weeksCache       map[string]scheduleCacheItem
	allGroups        []int32
	nowFunc          func() time.Time
}

func NewScheduleService(log *slog.Logger, scheduleRepo ScheduleRepository) *ScheduleService {
	return &ScheduleService{
		log:          log,
		scheduleRepo: scheduleRepo,
		weeksCache:   make(map[string]scheduleCacheItem),
	}
}

func (s *ScheduleService) now() time.Time {
	s.mu.RLock()
	fn := s.nowFunc
	s.mu.RUnlock()
	if fn != nil {
		return fn()
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

	now := s.now()
	targetWeek := EffectiveScheduleWeek(now)
	targetMonday := CurrentWeekMonday(targetWeek)

	s.mu.RLock()
	if s.cache != nil {
		var cachedWeek time.Time
		for _, g := range s.cache.Groups {
			if !g.Week.IsZero() {
				cachedWeek = g.Week
				break
			}
		}
		isSameTarget := (!cachedWeek.IsZero() && CurrentWeekMonday(cachedWeek).Equal(targetMonday)) ||
			(s.cachedTargetWeek.Equal(targetMonday) && time.Since(s.cacheAt) < scheduleCacheTTL)
		if isSameTarget {
			group, ok := s.cache.Groups[groupID]
			s.mu.RUnlock()
			if !ok {
				log.WarnContext(ctx, "group not found in cached schedule")
				return nil, domainerr.ErrGroupNotFound
			}
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

	s.setActiveCache(sch, targetMonday)

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

	weekKey := week.Format("2006-01-02")
	s.mu.RLock()
	if s.weeksCache != nil {
		if item, ok := s.weeksCache[weekKey]; ok && item.schedule != nil && time.Since(item.cachedAt) < scheduleCacheTTL {
			group, exists := item.schedule.Groups[groupID]
			s.mu.RUnlock()
			if !exists {
				return nil, domainerr.ErrGroupNotFound
			}
			return group, nil
		}
	}
	s.mu.RUnlock()

	sch, err := s.scheduleRepo.GetByWeek(ctx, week)
	if err != nil {
		if errors.Is(err, domainerr.ErrWeekNotFound) {
			log.WarnContext(ctx, "week not found")
			return nil, err
		}
		log.ErrorContext(ctx, "failed get by week", "error", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	s.mu.Lock()
	if s.weeksCache == nil {
		s.weeksCache = make(map[string]scheduleCacheItem)
	}
	now := time.Now()
	for k, v := range s.weeksCache {
		if now.Sub(v.cachedAt) >= scheduleCacheTTL {
			delete(s.weeksCache, k)
		}
	}
	s.weeksCache[weekKey] = scheduleCacheItem{
		schedule: sch,
		cachedAt: now,
	}
	s.mu.Unlock()

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

func (s *ScheduleService) setActiveCache(sch *entities.Schedule, targetMonday time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = sch
	s.cachedTargetWeek = targetMonday
	s.cacheAt = time.Now()
	s.mergeAllGroupsLocked(sch)
	if sch != nil {
		if s.weeksCache == nil {
			s.weeksCache = make(map[string]scheduleCacheItem)
		}
		for _, g := range sch.Groups {
			if !g.Week.IsZero() {
				s.weeksCache[g.Week.Format("2006-01-02")] = scheduleCacheItem{
					schedule: sch,
					cachedAt: s.cacheAt,
				}
				break
			}
		}
	}
}

func (s *ScheduleService) SetCache(sch *entities.Schedule) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cache = sch
	s.cachedTargetWeek = time.Time{}
	s.cacheAt = time.Now()
	s.mergeAllGroupsLocked(sch)
	// Сбрасываем кэш недель при обновлении расписания в БД
	s.weeksCache = make(map[string]scheduleCacheItem)
	if sch != nil {
		for _, g := range sch.Groups {
			if !g.Week.IsZero() {
				s.weeksCache[g.Week.Format("2006-01-02")] = scheduleCacheItem{
					schedule: sch,
					cachedAt: s.cacheAt,
				}
				break
			}
		}
	}
}

func (s *ScheduleService) mergeAllGroupsLocked(sch *entities.Schedule) {
	if sch == nil || len(s.allGroups) == 0 {
		return
	}
	var added bool
	groupSet := make(map[int32]struct{}, len(s.allGroups)+len(sch.Groups))
	for _, id := range s.allGroups {
		groupSet[id] = struct{}{}
	}
	for id := range sch.Groups {
		if _, ok := groupSet[id]; !ok {
			groupSet[id] = struct{}{}
			added = true
		}
	}
	if added {
		merged := make([]int32, 0, len(groupSet))
		for id := range groupSet {
			merged = append(merged, id)
		}
		slices.Sort(merged)
		s.allGroups = merged
	}
}

// ClearWeeksCache принудительно очищает in-memory кэш недель и активного расписания.
func (s *ScheduleService) ClearWeeksCache() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.weeksCache = make(map[string]scheduleCacheItem)
	s.cache = nil
	s.cachedTargetWeek = time.Time{}
}

func (s *ScheduleService) GetAllGroupIDs(ctx context.Context) ([]int32, error) {
	const op = "service.ScheduleService.GetAllGroupIDs"

	s.mu.RLock()
	if len(s.allGroups) > 0 {
		cp := slices.Clone(s.allGroups)
		s.mu.RUnlock()
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

	return slices.Clone(ids), nil
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
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	return grp, nil
}
