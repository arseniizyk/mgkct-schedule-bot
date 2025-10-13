package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/model"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/repository"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/service/parser"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/utils"
)

type Schedule interface {
	GetFullLatestSchedule(ctx context.Context) (*pb.Schedule, error)
	GetGroupLatestSchedule(ctx context.Context, groupID int32) (*pb.Group, error)
	GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*pb.Group, error)
	CheckScheduleUpdates(interval time.Duration) <-chan *model.Updated
	GetAvailableWeeks(ctx context.Context, week time.Time) (*model.Weeks, error)
}

type service struct {
	repo         repository.Schedule
	parser       *parser.Parser
	cache        *pb.Schedule
	scheduleHash [32]byte
	groupsHashes map[int32][32]byte
}

func NewScheduleService(scheduleRepo repository.Schedule) Schedule {
	return &service{
		repo:         scheduleRepo,
		parser:       parser.New(),
		groupsHashes: make(map[int32][32]byte),
	}
}

func (p *service) CheckScheduleUpdates(interval time.Duration) <-chan *model.Updated {
	resCh := make(chan *model.Updated, 1)

	go func() {
		tick := time.NewTicker(interval)
		defer tick.Stop()
		defer close(resCh)

		var sch *pb.Schedule
		var err error

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		sch, err = p.repo.GetLatest(ctx)
		if err != nil && errors.Is(err, model.ErrNotFound) {
			if sch, updated, err := p.parseSchedule(ctx); err == nil && updated {
				p.cache = sch
				updatedGroups := p.findUpdatedGroups(sch)
				for _, update := range updatedGroups {
					resCh <- update
				}
			}
		}
		if sch == nil {
			sch = &pb.Schedule{}
		}
		p.cache = sch
		p.hashGroups(sch)

		for range tick.C {
			sch, updated, err := p.parseSchedule(context.Background())
			if err != nil {
				slog.Error("checkscheduleupdates: parseSchedule", "err", err)
				continue
			}
			if !updated {
				slog.Debug("schedule wasn't updated")
				continue
			}

			updatedGroups := p.findUpdatedGroups(sch)
			for _, update := range updatedGroups {
				var oldWeek time.Time
				if group, ok := p.cache.Groups[update.Group.Id]; ok && group != nil {
					oldWeek = group.Week.AsTime()
				}
				newWeek := update.Group.Week.AsTime()

				if !oldWeek.IsZero() && !oldWeek.Equal(newWeek) {
					update.IsWeekUpdated = true
					resCh <- update
					break
				}

				slog.Info("Group updated", "group_id", update.Group.Id)
				resCh <- update
			}
			p.cache = sch
			p.hashGroups(sch)
		}
	}()

	return resCh
}

func (p *service) findUpdatedGroups(new *pb.Schedule) []*model.Updated {
	updated := make([]*model.Updated, 0, 1)
	for key, group := range new.Groups {
		newGroupHash, err := utils.HashJSON(group)
		if err != nil {
			slog.Error("groupHash failed", "group", key, "err", err)
			continue
		}
		if p.groupsHashes[key] != newGroupHash {
			p.groupsHashes[key] = newGroupHash
			updated = append(updated, &model.Updated{
				Group: group,
			})
		}
	}

	return updated
}

func (p *service) hashGroups(sch *pb.Schedule) {
	for num, g := range sch.Groups {
		if h, err := utils.HashJSON(g); err == nil {
			p.groupsHashes[num] = h
		} else {
			slog.Error("group hash failed", "group", num, "err", err)
		}
	}
}

func (p *service) parseSchedule(ctx context.Context) (*pb.Schedule, bool, error) {
	start := time.Now()
	slog.Debug("parsing")
	defer func() {
		slog.Debug("parsed", "duration", time.Since(start))
	}()

	sch, week, err := p.parser.Parse()
	if err != nil {
		return nil, false, fmt.Errorf("parsing: %w", err)
	}

	h, err := utils.HashJSON(sch)
	if err != nil {
		return nil, false, fmt.Errorf("can't get hash for schedule: %w", err)
	}

	if h == p.scheduleHash { // if previous hash schedule == parsed hash schedule
		return nil, false, nil
	}

	p.scheduleHash = h
	if err := p.repo.Save(ctx, *week, sch); err != nil {
		return nil, false, fmt.Errorf("save to db: %w", err)
	}

	return sch, true, nil
}
