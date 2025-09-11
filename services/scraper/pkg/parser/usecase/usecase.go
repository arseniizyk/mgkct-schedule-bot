package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/database"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/parser"
)

type ParserUsecase interface {
	GetScheduleEvery(ctx context.Context, interval time.Duration) <-chan *models.Schedule
}

type parserUsecase struct {
	db     database.DatabaseRepository
	parser *parser.Parser
}

func NewParserUsecase(db database.DatabaseRepository, p *parser.Parser) ParserUsecase {
	return &parserUsecase{
		db:     db,
		parser: p,
	}
}

func (p *parserUsecase) GetScheduleEvery(ctx context.Context, interval time.Duration) <-chan *models.Schedule {
	resCh := make(chan *models.Schedule)
	var prev [32]byte

	go func() {
		tick := time.NewTicker(1 * time.Minute)
		defer tick.Stop()
		defer close(resCh)

		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				start := time.Now()
				slog.Info("parsing")

				sch, week, err := p.parser.Parse()
				if err != nil {
					slog.Error("parsing error:", "err", err)
					continue
				}

				if hash(sch) != prev {
					prev = hash(sch)
					if err := p.db.SaveSchedule(ctx, *week, sch); err != nil {
						slog.Error("can't save schedule to database", "err", err)
						continue
					}
					resCh <- sch
				}

				slog.Info("parsed", "duration", time.Since(start))
			}
		}

	}()

	return resCh
}

func hash(sch *models.Schedule) [32]byte {
	bytes, _ := json.Marshal(sch)
	return sha256.Sum256(bytes)
}
