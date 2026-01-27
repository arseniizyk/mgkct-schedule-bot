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
	"google.golang.org/protobuf/proto"
)

type ScheduleService interface {
	GetGroupLatestSchedule(ctx context.Context, groupID int32) (*pb.Group, error)
	GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*pb.Group, error)
	CheckScheduleUpdates(ctx context.Context, interval time.Duration) <-chan *entities.UpdatedGroup
}

type ScheduleTransport struct {
	log             *slog.Logger
	scheduleService ScheduleService
	natsConn        *nats.Conn
}

func NewScheduleTransport(log *slog.Logger, scheduleService ScheduleService, natsConn *nats.Conn) *ScheduleTransport {
	return &ScheduleTransport{
		log:             log,
		scheduleService: scheduleService,
		natsConn:        natsConn,
	}
}

func (t *ScheduleTransport) GetGroupSchedule(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	sch, err := t.scheduleService.GetGroupLatestSchedule(ctx, req.Id)
	if err != nil {
		if errors.Is(err, repository.ErrGroupNotFound) {
			return nil, status.Errorf(codes.NotFound, "group not found")
		}

		if errors.Is(err, repository.ErrScheduleNotFound) {
			return nil, status.Errorf(codes.NotFound, "schedule not found, internal error")
		}

		return nil, status.Errorf(codes.Internal, "failed to get group schedule")
	}

	return &pb.GroupScheduleResponse{
		Group: sch,
	}, nil
}

func (t *ScheduleTransport) GetGroupScheduleByWeek(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	group, err := t.scheduleService.GetGroupScheduleByWeek(ctx, req.Id, req.Week.AsTime())
	if err != nil {
		if errors.Is(err, repository.ErrGroupNotFound) {
			return nil, status.Errorf(codes.NotFound, "group not found")
		}

		if errors.Is(err, repository.ErrWeekNotFound) {
			return nil, status.Errorf(codes.NotFound, "week not found")
		}
		return nil, status.Errorf(codes.Internal, "can't get schedule")
	}

	return &pb.GroupScheduleResponse{
		Group: group,
	}, nil
}

func (t *ScheduleTransport) PublishScheduleUpdate(group *pb.Group) error {
	log := t.log.With(
		"operation", "transport.schedule.ScheduleTransport.PublishScheduleUpdate",
		"group_id", group.Id,
		"week", group.Week.String(),
	)

	log.Info("Publishing schedule update")
	resp := &pb.GroupScheduleResponse{Group: group}

	data, err := proto.Marshal(resp)
	if err != nil {
		log.Error("failed marshalling proto", "error", err)
		return err
	}

	if err := t.natsConn.Publish("schedule.updates", data); err != nil {
		log.Error("failed publishing schedule update to nats", "error", err)
		return err
	}

	return nil
}
