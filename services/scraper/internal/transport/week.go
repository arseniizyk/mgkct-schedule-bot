package transport

import (
	"context"
	"errors"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/repository"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type WeekService interface {
	GetAvailableWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error)
}

type WeekTransport struct {
	log         *slog.Logger
	weekService WeekService
	natsConn    *nats.Conn
}

func NewWeekTransport(log *slog.Logger, weekService WeekService, natsConn *nats.Conn) *WeekTransport {
	return &WeekTransport{
		log:         log,
		weekService: weekService,
		natsConn:    natsConn,
	}
}

func (t *WeekTransport) GetAvailableWeeks(ctx context.Context, req *pb.AvailableWeeksRequest) (*pb.AvailableWeeksResponse, error) {
	var week time.Time
	if req.Week != nil {
		week = req.Week.AsTime()
	}

	weeks, err := t.weekService.GetAvailableWeeks(ctx, week)
	if err != nil {
		if errors.Is(err, repository.ErrNoAvailableWeeks) {
			return nil, status.Errorf(codes.NotFound, "no available weeks")
		}
		return nil, status.Errorf(codes.Internal, "can't get weeks: %v", err)
	}

	return &pb.AvailableWeeksResponse{
		Prev:    timestamppb.New(weeks.Prev),
		Current: timestamppb.New(weeks.Current),
		Next:    timestamppb.New(weeks.Next),
	}, nil
}

func (t *WeekTransport) PublishWeekUpdates(date time.Time) error {
	log := t.log.With(
		"operation", "transport.week.WeekTransport.PublishUpdatedWeekDate",
		"date", date.Format("2006-01-02"),
	)

	log.Info("publishing updated week date")

	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	data := []byte(date.Format(time.RFC3339))

	if err := t.natsConn.Publish("schedule.week.updates", data); err != nil {
		log.Error("failed to publish updated week date", "error", err)
		return err
	}

	return nil
}
