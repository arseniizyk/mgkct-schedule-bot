package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule"
	server "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/transport"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/parser"
)

type ParserUsecase interface {
	ParseScheduleEvery(ctx context.Context, interval time.Duration) <-chan *models.Schedule
}

type parserUsecase struct {
	schUC    schedule.ScheduleUsecase
	parser   *parser.Parser
	hashes   map[int][32]byte
	prevHash [32]byte
	nats     *server.Nats
}

func NewParserUsecase(schUC schedule.ScheduleUsecase, p *parser.Parser, nats *server.Nats) ParserUsecase {
	return &parserUsecase{
		schUC:  schUC,
		parser: p,
		nats:   nats,
		hashes: make(map[int][32]byte),
	}
}

func (p *parserUsecase) ParseScheduleEvery(ctx context.Context, interval time.Duration) <-chan *models.Schedule {
	resCh := make(chan *models.Schedule)

	go func() {
		tick := time.NewTicker(interval)
		defer tick.Stop()
		defer close(resCh)

		if sch, updated, err := p.parseSchedule(ctx); err == nil && updated {
			for num, g := range sch.Groups {
				if h, err := hash(g); err == nil {
					p.hashes[num] = h
				} else {
					slog.Error("group hash failed", "group", num, "err", err)
				}
			}
			resCh <- sch
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				sch, updated, err := p.parseSchedule(ctx)
				if err != nil || !updated {
					continue
				}
				if updated {
					updatedGroups := p.findUpdatedGroups(sch)
					for _, group := range updatedGroups {
						p.nats.PublishScheduleUpdate(&group)
					}
					resCh <- sch
				}
			}
		}
	}()

	return resCh
}

func (p *parserUsecase) findUpdatedGroups(new *models.Schedule) []models.Group {
	updated := make([]models.Group, 0, 1)
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

func (p *parserUsecase) parseSchedule(ctx context.Context) (*models.Schedule, bool, error) {
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
