package transport

import (
	"context"
	"errors"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/model"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/repository"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type ScheduleService interface {
	GetGroupLatestSchedule(ctx context.Context, groupID int32) (*pb.Group, error)
	GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*pb.Group, error)
	CheckScheduleUpdates(interval time.Duration) <-chan *model.Updated
	GetAvailableWeeks(ctx context.Context, week time.Time) (*model.Weeks, error)
}

type transport struct {
	service ScheduleService
	nc      *nats.Conn
	pb.UnimplementedScheduleServiceServer
}

func NewScheduleTransport(service ScheduleService, nc *nats.Conn) *transport {
	return &transport{
		service: service,
		nc:      nc,
	}
}

func (t *transport) GetGroupSchedule(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	sch, err := t.service.GetGroupLatestSchedule(ctx, req.Id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "group not found")
		}
		return nil, status.Errorf(codes.Unavailable, "can't get schedule")
	}

	return &pb.GroupScheduleResponse{
		Group: sch,
	}, nil
}

func (t *transport) GetGroupScheduleByWeek(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	group, err := t.service.GetGroupScheduleByWeek(ctx, req.Id, req.Week.AsTime())
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "group not found")
		}
		return nil, status.Errorf(codes.Unavailable, "can't get schedule")
	}

	return &pb.GroupScheduleResponse{
		Group: group,
	}, nil
}

func (t *transport) GetAvailableWeeks(ctx context.Context, req *pb.AvailableWeeksRequest) (*pb.AvailableWeeksResponse, error) {
	var week time.Time
	if req.Week != nil {
		week = req.Week.AsTime()
	}

	weeks, err := t.service.GetAvailableWeeks(ctx, week)
	if err != nil {
		if errors.Is(err, repository.ErrNoAvailableWeeks) {
			return nil, status.Errorf(codes.NotFound, "%s", err.Error())
		}
		return nil, status.Errorf(codes.Internal, "can't get weeks: %v", err)
	}

	return &pb.AvailableWeeksResponse{
		Prev:    timestamppb.New(weeks.Prev),
		Current: timestamppb.New(weeks.Current),
		Next:    timestamppb.New(weeks.Next),
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

func (t *transport) PublishWeekUpdates(date time.Time) error {
	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	slog.Debug("Publishing week update", "date", date.Format("2006-01-02"))
	data := []byte(date.Format(time.RFC3339))
	if err := t.nc.Publish("schedule.week.updates", data); err != nil {
		slog.Error("PublishWeekUpdate: failed to publish", "date", date.Format("2006-01-02"), "err", err)
		return err
	}
	return nil
}
