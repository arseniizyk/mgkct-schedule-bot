package week

import (
	"context"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/errors"
)

type WeekTransport struct {
	log *slog.Logger

	natsConn    *nats.Conn
	weekService WeekService
}

type WeekService interface {
	GetAvailableWeeks(ctx context.Context, in *pb.AvailableWeeksRequest, opts ...grpc.CallOption) (*pb.AvailableWeeksResponse, error)
}

func New(log *slog.Logger, natsConn *nats.Conn, weekService WeekService) *WeekTransport {
	return &WeekTransport{
		log:         log,
		natsConn:    natsConn,
		weekService: weekService,
	}
}

func (t *WeekTransport) GetAvailableWeeks(ctx context.Context, week *time.Time) (entities.Weeks, error) {
	var w *timestamppb.Timestamp
	if week != nil {
		w = timestamppb.New(*week)
	}

	log := t.log.With(
		"operation", "transport.grpc.week.WeekTransport.GetAvailableWeeks",
		"week", w.String(),
	)

	resp, err := t.weekService.GetAvailableWeeks(ctx, &pb.AvailableWeeksRequest{Week: w})
	if err != nil {
		s, ok := status.FromError(err)
		if !ok {
			log.ErrorContext(ctx, "failed getting status from error", "error", err)
			return entities.Weeks{}, domainerr.ErrServiceInternal
		}

		switch s.Code() {
		case codes.NotFound:
			log.WarnContext(ctx, "weeks not found")
			return entities.Weeks{}, domainerr.ErrWeekNotFound
		default:
			log.ErrorContext(ctx, "unexpected status code from service", "status_code", s.Code())
			return entities.Weeks{}, domainerr.ErrServiceInternal
		}
	}

	res := entities.Weeks{Current: resp.GetCurrent().AsTime()}

	if resp.GetPrev().IsValid() && !resp.GetPrev().AsTime().IsZero() {
		res.Prev = resp.GetPrev().AsTime()
	}

	if resp.GetNext().IsValid() && !resp.GetNext().AsTime().IsZero() {
		res.Next = resp.GetNext().AsTime()
	}

	return res, nil
}
