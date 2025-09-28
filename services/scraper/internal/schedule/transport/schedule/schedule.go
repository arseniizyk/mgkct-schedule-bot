package schedule

import (
	"context"
	"errors"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/model"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/service"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type transport struct {
	service service.ScheduleService
	nc      *nats.Conn
	pb.UnimplementedScheduleServiceServer
}

func New(service service.ScheduleService, nc *nats.Conn) *transport {
	return &transport{
		service: service,
		nc:      nc,
	}
}

func (t *transport) GetGroupSchedule(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	sch, err := t.service.GetFullLatestSchedule(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "can't get schedule")
	}

	group, ok := sch.Groups[req.Id]
	if !ok {
		return nil, status.Error(codes.NotFound, "group not found")
	}
	return &pb.GroupScheduleResponse{
		Group: group,
	}, nil
}

func (t *transport) GetGroupScheduleByWeek(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	group, err := t.service.GetGroupScheduleByWeek(ctx, req.Id, req.Week.AsTime())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "group not found")
		}
		return nil, status.Errorf(codes.Unavailable, "can't get schedule")
	}

	return &pb.GroupScheduleResponse{
		Group: group,
	}, nil
}

func (t *transport) PublishScheduleUpdate(group *pb.Group) error {
	slog.Debug("Publishing schedule update", "group_id", group.Id)
	resp := &pb.GroupScheduleResponse{
		Group: group,
	}
	data, err := proto.Marshal(resp)
	if err != nil {
		slog.Error("PublishScheduleUpdate: marshal proto", "group", group, "err", err)
		return err
	}
	return t.nc.Publish("schedule.updates", data)
}
