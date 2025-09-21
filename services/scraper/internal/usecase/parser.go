package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/parser"
)

type ParserUsecase interface {
	ParseScheduleEvery(ctx context.Context, interval time.Duration) <-chan *models.Schedule
}

type parserUsecase struct {
	schUC    schedule.ScheduleUsecase
	parser   *parser.Parser
	prevHash [32]byte
}

func NewParserUsecase(schUC schedule.ScheduleUsecase, p *parser.Parser) ParserUsecase {
	return &parserUsecase{
		schUC:  schUC,
		parser: p,
	}
}

func (p *parserUsecase) ParseScheduleEvery(ctx context.Context, interval time.Duration) <-chan *models.Schedule {
	resCh := make(chan *models.Schedule)

	go func() {
		tick := time.NewTicker(interval)
		defer tick.Stop()
		defer close(resCh)

		sch, updated, err := p.parseSchedule(ctx)
		if err == nil && updated {
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
					resCh <- sch
				}
			}
		}
	}()

	return resCh
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

	if hash(sch) == p.prevHash { // if previous hash schedule == parsed hash schedule
		return nil, false, nil
	}

	p.prevHash = hash(sch)
	if err := p.schUC.Save(ctx, *week, sch); err != nil {
		return nil, false, fmt.Errorf("save to db: %w", err)
	}

	return sch, true, nil

}

func hash(sch *models.Schedule) [32]byte {
	bytes, _ := json.Marshal(sch)
	return sha256.Sum256(bytes)
}
