package schedule

import (
	"context"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type transport struct {
	nc         *nats.Conn
	scraperSvc pb.ScheduleServiceClient
}

func New(nc *nats.Conn, scraperSvc pb.ScheduleServiceClient) *transport {
	return &transport{
		nc:         nc,
		scraperSvc: scraperSvc,
	}
}

func (t *transport) GetGroupSchedule(ctx context.Context, groupID int) (*pb.Group, error) {
	resp, err := t.scraperSvc.GetGroupSchedule(ctx, &pb.GroupScheduleRequest{
		Id: int32(groupID),
	})

	if err != nil {
		return nil, t.handleGRPCError(groupID, err)
	}

	return resp.Group, nil
}

func (t *transport) GetGroupScheduleByWeek(ctx context.Context, groupID int, week time.Time) (*pb.Group, error) {
	resp, err := t.scraperSvc.GetGroupSchedule(ctx, &pb.GroupScheduleRequest{
		Id:   int32(groupID),
		Week: timestamppb.New(week),
	})

	if err != nil {
		return nil, t.handleGRPCError(groupID, err)
	}

	return resp.Group, nil
}

func (t *transport) handleGRPCError(groupID int, err error) error {
	st, ok := status.FromError(err)
	if !ok {
		slog.Error("undefined error from scraper", "group_id", groupID, "err", err)
		return models.ErrScraperInternal
	}

	switch st.Code() {
	case codes.NotFound:
		slog.Warn("group not found from scraper", "err", st.Message(), "code", st.Code(), "group_id", groupID)
		return models.ErrGroupNotFound
	case codes.Unavailable:
		slog.Error("scraper unavailable", "err", st.Message(), "code", st.Code(), "group_id", groupID)
		return models.ErrScraperInternal
	default:
		slog.Error("undefined error from scraper", "err", st.Message(), "code", st.Code(), "group_id", groupID)
		return models.ErrScraperInternal
	}
}
