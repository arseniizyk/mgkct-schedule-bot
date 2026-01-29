package schedule

import (
	"context"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/errors"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ScheduleService interface {
	GetGroupSchedule(ctx context.Context, req *pb.GroupScheduleRequest, opts ...grpc.CallOption) (*pb.GroupScheduleResponse, error)
	GetGroupScheduleByWeek(ctx context.Context, req *pb.GroupScheduleRequest, opts ...grpc.CallOption) (*pb.GroupScheduleResponse, error)
}

type ScheduleTransport struct {
	log *slog.Logger

	natsConn        *nats.Conn
	scheduleService ScheduleService
}

func New(log *slog.Logger, natsConn *nats.Conn, scheduleService ScheduleService) *ScheduleTransport {
	return &ScheduleTransport{
		log:             log,
		natsConn:        natsConn,
		scheduleService: scheduleService,
	}
}

func (t *ScheduleTransport) GetGroupSchedule(ctx context.Context, groupID int) (*pb.Group, error) {
	log := t.log.With(
		"operation", "transport.grpc.schedule.ScheduleTransport.GetGroupSchedule",
		"group_id", groupID,
	)

	resp, err := t.scheduleService.GetGroupSchedule(ctx, &pb.GroupScheduleRequest{
		Id: int32(groupID),
	})

	if err != nil {
		s, ok := status.FromError(err)
		if !ok {
			log.Error("Undefined gRPC err, failed on getting status", "error", err)
			return nil, domainerr.ErrServiceInternal
		}

		switch s.Code() {
		case codes.NotFound:
			switch s.Message() {
			case "group not found":
				log.Warn("Group not found")
				return nil, domainerr.ErrGroupNotFound

			case "schedule not found":
				log.Warn("Schedule not found")
				return nil, domainerr.ErrScheduleNotFound

			default:
				log.Error("Service internal status message", "message", s.Message())
				return nil, domainerr.ErrServiceInternal
			}

		default:
			log.Error("Unexpected gRPC status code", "status", s.Code(), "message", s.Message())
			return nil, domainerr.ErrServiceInternal
		}
	}

	return resp.Group, nil
}

func (t *ScheduleTransport) GetGroupScheduleByWeek(ctx context.Context, groupID int, week time.Time) (*pb.Group, error) {
	log := t.log.With(
		"operation", "transport.grpc.schedule.ScheduleTransport.GetGroupScheduleByWeek",
		"group_id", groupID,
		"week", week.String(),
	)

	resp, err := t.scheduleService.GetGroupScheduleByWeek(ctx, &pb.GroupScheduleRequest{
		Id:   int32(groupID),
		Week: timestamppb.New(week),
	})

	if err != nil {
		s, ok := status.FromError(err)
		if !ok {
			log.Error("Undefined gRPC err, failed on getting status", "error", err)
			return nil, domainerr.ErrServiceInternal
		}

		switch s.Code() {
		case codes.NotFound:
			switch s.Message() {
			case "group not found":
				log.Warn("Group not found")
				return nil, domainerr.ErrGroupNotFound

			case "week not found":
				log.Warn("Week not found")
				return nil, domainerr.ErrWeekNotFound

			default:
				log.Error("Service internal status message", "message", s.Message())
				return nil, domainerr.ErrServiceInternal
			}

		default:
			log.Error("Unexpected gRPC status code", "status", s.Code(), "message", s.Message())
			return nil, domainerr.ErrServiceInternal
		}
	}

	return resp.Group, nil
}
