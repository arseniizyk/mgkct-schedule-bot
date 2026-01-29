package week

import (
	"context"
	"log/slog"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
)

type WeekTransport interface {
	GetAvailableWeeks(ctx context.Context, week *time.Time) (entities.Weeks, error)
}

type WeekUsecase struct {
	log           *slog.Logger
	weekTransport WeekTransport
}

func New(log *slog.Logger, weekTransport WeekTransport) *WeekUsecase {
	return &WeekUsecase{
		log:           log,
		weekTransport: weekTransport,
	}
}

func (w *WeekUsecase) GetAvailableWeeks(ctx context.Context, week *time.Time) (entities.Weeks, error) {
	var weekLog string

	if week == nil {
		weekLog = "nil"
	} else {
		weekLog = week.String()
	}

	log := w.log.With(
		"operation", "usecases.week.GetAvailableWeeks",
		"week", weekLog,
	)

	resp, err := w.weekTransport.GetAvailableWeeks(ctx, week)
	if err != nil {
		log.Error("failed get available weeks:", "err", err)
		return entities.Weeks{}, err
	}

	return resp, nil
}
