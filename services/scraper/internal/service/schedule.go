package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/repository"
)

type ScheduleRepository interface {
	Save(ctx context.Context, week time.Time, schedule *pb.Schedule) error
	GetByWeek(ctx context.Context, week time.Time) (*pb.Schedule, error)
	GetLatest(ctx context.Context) (*pb.Schedule, error)
	GetWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error)
}

type Parser interface {
	Parse(ctx context.Context) (schedule *pb.Schedule, week *time.Time, err error)
}

type ScheduleService struct {
	log *slog.Logger

	scheduleRepo ScheduleRepository
	parser       Parser

	mu           sync.RWMutex
	cache        *pb.Schedule
	scheduleHash [32]byte
	groupHashes  map[int32][32]byte
}

func NewScheduleService(log *slog.Logger, scheduleRepo ScheduleRepository, parser Parser) *ScheduleService {
	return &ScheduleService{
		log:          log,
		scheduleRepo: scheduleRepo,
		parser:       parser,
		groupHashes:  make(map[int32][32]byte),
	}
}

func (ss *ScheduleService) GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*pb.Group, error) {
	const op = "service.schedule.ScheduleService.GetGroupScheduleByWeek"
	log := ss.log.With(
		"operation", op,
		"group_id", groupID,
		"week", week,
	)

	sch, err := ss.scheduleRepo.GetByWeek(ctx, week)
	if err != nil {
		if errors.Is(err, repository.ErrWeekNotFound) {
			log.ErrorContext(ctx, "week not found")
			return nil, err
		}
		log.ErrorContext(ctx, "failed get by week", "err", err)
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	group, ok := sch.GetGroups()[groupID]
	if !ok {
		log.ErrorContext(ctx, "group not found", "err", err)
		return nil, repository.ErrGroupNotFound
	}

	return group, nil
}

func (ss *ScheduleService) GetGroupLatestSchedule(ctx context.Context, groupID int32) (*pb.Group, error) {
	const op = "service.schedule.ScheduleService.GetGroupLatestSchedule"

	log := ss.log.With("operation", op, "group_id", groupID)

	if ss.cache != nil {
		ss.mu.RLock()
		group, ok := ss.cache.GetGroups()[groupID]
		ss.mu.RUnlock()
		if ok {
			return group, nil
		}
		log.WarnContext(ctx, "can't get group from cache")
	}

	sch, err := ss.scheduleRepo.GetLatest(ctx)
	if err != nil {
		if errors.Is(err, repository.ErrScheduleNotFound) {
			log.ErrorContext(ctx, "schedule not found in repository", "error", err)
			return nil, err
		}
		log.ErrorContext(ctx, "failed get by latest", "error", err)
		return nil, fmt.Errorf("get by latest error: %w", err)
	}

	group, ok := sch.GetGroups()[groupID]
	if !ok {
		log.WarnContext(ctx, "group not found", "err", err)
		return nil, repository.ErrGroupNotFound
	}

	return group, nil
}

func (ss *ScheduleService) CheckScheduleUpdates(ctx context.Context, interval time.Duration) <-chan *entities.UpdatedGroup {
	const op = "service.schedule.ScheduleService.CheckScheduleUpdates"

	log := ss.log.With("operation", op, "interval", interval.Seconds())

	resCh := make(chan *entities.UpdatedGroup, 5)

	go func() {
		tick := time.NewTicker(interval)
		defer tick.Stop()
		defer close(resCh)

		var sch *pb.Schedule

		repoCtx, cancel := context.WithTimeout(ctx, defaultCtxTimeout)
		sch, _ = ss.scheduleRepo.GetLatest(repoCtx)
		cancel()

		if sch == nil {
			sch = &pb.Schedule{}
		}

		ss.storeCache(sch)

		parseCtx, parseCancel := context.WithTimeout(ctx, defaultCtxTimeout)
		sch, updated, err := ss.parseSchedule(parseCtx)
		parseCancel()

		if err != nil {
			log.ErrorContext(ctx, "failed to parse schedule on startup", "error", err)
		} else if updated {
			updatedGroups := ss.findUpdatedGroups(sch)
			for _, update := range updatedGroups {
				resCh <- update
			}
			ss.storeCache(sch)
			log.InfoContext(ctx, "schedule updated on start")
		}

		for {
			select {
			case <-tick.C:
				parseCtx, cancel := context.WithTimeout(ctx, defaultCtxTimeout)
				sch, updated, err := ss.parseSchedule(parseCtx)
				cancel()

				if err != nil {
					log.ErrorContext(ctx, "failed on parsing schedule", "error", err)
					continue
				}
				if !updated {
					log.InfoContext(ctx, "schedule wasn't updated after parsing")
					continue
				}

				updatedGroups := ss.findUpdatedGroups(sch)
				for _, updatedGroup := range updatedGroups {
					var oldWeek time.Time
					if group, ok := ss.cache.GetGroups()[updatedGroup.Group.GetId()]; ok && group != nil {
						oldWeek = group.GetWeek().AsTime()
					}
					newWeek := updatedGroup.Group.GetWeek().AsTime()

					if !oldWeek.IsZero() && !oldWeek.Equal(newWeek) {
						updatedGroup.IsWeekUpdated = true
					}

					log.InfoContext(ctx, "group updated", "group_id", updatedGroup.Group.GetId(), "is_week_updated", updatedGroup.IsWeekUpdated)
					resCh <- updatedGroup
				}

				ss.storeCache(sch)

			case <-ctx.Done():
				log.InfoContext(ctx, "ctx.Done Received")
				return
			}
		}
	}()

	return resCh
}

func (ss *ScheduleService) findUpdatedGroups(next *pb.Schedule) []*entities.UpdatedGroup {
	const op = "services.schedule.ScheduleService.findUpdatedGroups"

	log := ss.log.With("operation", op)

	updated := make([]*entities.UpdatedGroup, 0, 1)
	for groupID, group := range next.GetGroups() {
		newGroupHash, err := hashJSON(group)
		if err != nil {
			log.Error("failed to hash group", "group", groupID, "err", err)
			continue
		}

		if ss.groupHashes[groupID] != newGroupHash {
			ss.groupHashes[groupID] = newGroupHash
			updated = append(updated, &entities.UpdatedGroup{Group: group})
		}
	}

	return updated
}

func (ss *ScheduleService) hashGroups(sch *pb.Schedule) {
	for num, g := range sch.GetGroups() {
		if h, err := hashJSON(g); err == nil {
			ss.groupHashes[num] = h
		} else {
			ss.log.Error("failed to hash groups", "group", num, "err", err)
		}
	}
}

func (ss *ScheduleService) storeCache(sch *pb.Schedule) {
	ss.mu.Lock()
	ss.cache = sch
	ss.hashGroups(sch)
	ss.mu.Unlock()
}

func (ss *ScheduleService) parseSchedule(ctx context.Context) (*pb.Schedule, bool, error) {
	const op = "services.schedule.ScheduleService.parseSchedule"

	log := ss.log.With("operation", op)

	start := time.Now()
	log.DebugContext(ctx, "Parsing Schedule")
	defer func() {
		log.DebugContext(ctx, "Schedule Parsed", "duration", time.Since(start))
	}()

	sch, week, err := ss.parser.Parse(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("%s: parsing failed: %w", op, err)
	}

	h, err := hashJSON(sch)
	if err != nil {
		return nil, false, fmt.Errorf("%s: can't hash schedule: %w", op, err)
	}

	if h == ss.scheduleHash {
		return nil, false, nil
	}

	if err := ss.scheduleRepo.Save(ctx, *week, sch); err != nil {
		return nil, false, fmt.Errorf("%s: failed to save schedule in repository: %w", op, err)
	}

	ss.scheduleHash = h
	return sch, true, nil
}

func hashJSON[T any](t T) ([32]byte, error) {
	var zero [32]byte
	b, err := json.Marshal(t)
	if err != nil {
		return zero, fmt.Errorf("failed to marshal json: %w", err)
	}
	return sha256.Sum256(b), nil
}
