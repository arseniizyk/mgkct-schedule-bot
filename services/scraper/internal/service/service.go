package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/repository/postgres"
	server "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/transport"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/parser"
)

type ParserService struct {
	schUC    schedule.ScheduleUsecase
	parser   *parser.Parser
	hashes   map[int32][32]byte
	prevHash [32]byte
	nats     *server.Nats
}

func NewParserService(schUC schedule.ScheduleUsecase, p *parser.Parser, nats *server.Nats) *ParserService {
	return &ParserService{
		schUC:  schUC,
		parser: p,
		nats:   nats,
		hashes: make(map[int32][32]byte),
	}
}

func (p *ParserService) ParseScheduleEvery(ctx context.Context, interval time.Duration) <-chan *pb.Schedule {
	resCh := make(chan *pb.Schedule, 1)

	go func() {
		tick := time.NewTicker(interval)
		defer tick.Stop()
		defer close(resCh)

		var sch *pb.Schedule
		var err error

		sch, err = p.schUC.GetLatest()
		if err != nil && errors.Is(err, postgres.ErrNotFound) {
			if sch, updated, err := p.parseSchedule(ctx); err == nil && updated {
				resCh <- sch
			}
		}
		if sch == nil {
			sch = &pb.Schedule{}
		}
		p.hashGroups(sch)

		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				sch, updated, err := p.parseSchedule(ctx)
				if err != nil || !updated {
					continue
				}
				updatedGroups := p.findUpdatedGroups(sch)
				for _, group := range updatedGroups {
					slog.Info("Group updated", "group_id", group.Id)
					if err := p.nats.PublishScheduleUpdate(group); err != nil {
						slog.Error("Publish to NATS", "group_id", group.Id, "err", err)
					}
				}
				p.hashGroups(sch)
				resCh <- sch
			}
		}
	}()

	return resCh
}

func (p *ParserService) findUpdatedGroups(new *pb.Schedule) []*pb.Group {
	updated := make([]*pb.Group, 0, 1)
	for key, group := range new.Groups {
		newHash, err := hash(group)
		if err != nil {
			slog.Error("groupHash failed", "group", key, "err", err)
			updated = append(updated, group)
			continue
		}
		if p.hashes[key] != newHash {
			p.hashes[key] = newHash
			updated = append(updated, group)
		}
	}

	return updated
}

func (p *ParserService) hashGroups(sch *pb.Schedule) {
	for num, g := range sch.Groups {
		if h, err := hash(g); err == nil {
			p.hashes[num] = h
		} else {
			slog.Error("group hash failed", "group", num, "err", err)
		}
	}
}

func (p *ParserService) parseSchedule(ctx context.Context) (*pb.Schedule, bool, error) {
	start := time.Now()
	slog.Debug("parsing")
	defer func() {
		slog.Debug("parsed", "duration", time.Since(start))
	}()

	sch, week, err := p.parser.Parse()
	if err != nil {
		return nil, false, fmt.Errorf("parsing: %w", err)
	}

	h, err := hash(sch)
	if err != nil {
		return nil, false, fmt.Errorf("can't get hash for schedule: %w", err)
	}

	if h == p.prevHash { // if previous hash schedule == parsed hash schedule
		return nil, false, nil
	}

	p.prevHash = h
	if err := p.schUC.Save(ctx, *week, sch); err != nil {
		return nil, false, fmt.Errorf("save to db: %w", err)
	}

	return sch, true, nil

}

func hash[T any](sch T) ([32]byte, error) {
	var zero [32]byte
	b, err := json.Marshal(sch)
	if err != nil {
		slog.Error("hash: can't marshal", "sch", sch, "err", err)
		return zero, fmt.Errorf("hash: marshal: %w", err)
	}
	return sha256.Sum256(b), nil
}
