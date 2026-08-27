package transport

import (
	"context"
	"errors"
	"log/slog"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/nats-io/nats.go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/repository"
)

type TeacherScheduleService interface {
	GetTeacherLatestSchedule(ctx context.Context, name string) (*pb.Teacher, error)
	GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*pb.Teacher, error)
	GetAllTeacherNames(ctx context.Context) ([]string, error)
	GetAvailableWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error)
}

type TeacherScheduleTransport struct {
	log             *slog.Logger
	scheduleService TeacherScheduleService
	natsConn        *nats.Conn
}

func NewTeacherScheduleTransport(log *slog.Logger, scheduleService TeacherScheduleService, natsConn *nats.Conn) *TeacherScheduleTransport {
	return &TeacherScheduleTransport{
		log:             log,
		scheduleService: scheduleService,
		natsConn:        natsConn,
	}
}

func (t *TeacherScheduleTransport) GetTeacherSchedule(ctx context.Context, req *pb.TeacherScheduleRequest) (*pb.TeacherScheduleResponse, error) {
	teacher, err := t.scheduleService.GetTeacherLatestSchedule(ctx, req.GetName())
	if err != nil {
		if errors.Is(err, repository.ErrScheduleNotFound) {
			return nil, status.Errorf(codes.NotFound, "teacher schedule not found")
		}

		return nil, status.Errorf(codes.Internal, "failed to get teacher schedule")
	}

	return &pb.TeacherScheduleResponse{
		Teacher: teacher,
	}, nil
}

func (t *TeacherScheduleTransport) GetTeacherScheduleByWeek(ctx context.Context, req *pb.TeacherScheduleRequest) (*pb.TeacherScheduleResponse, error) {
	teacher, err := t.scheduleService.GetTeacherScheduleByWeek(ctx, req.GetName(), req.GetWeek().AsTime())
	if err != nil {
		if errors.Is(err, repository.ErrWeekNotFound) {
			return nil, status.Errorf(codes.NotFound, "week not found")
		}
		return nil, status.Errorf(codes.Internal, "can't get teacher schedule")
	}

	return &pb.TeacherScheduleResponse{
		Teacher: teacher,
	}, nil
}

func (t *TeacherScheduleTransport) GetAvailableTeacherWeeks(ctx context.Context, req *pb.AvailableWeeksRequest) (*pb.AvailableWeeksResponse, error) {
	var week time.Time
	if req.GetWeek() != nil {
		week = req.GetWeek().AsTime()
	}

	weeks, err := t.scheduleService.GetAvailableWeeks(ctx, "", week)
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

func (t *TeacherScheduleTransport) GetTeacherNames(ctx context.Context, _ *pb.Empty) (*pb.TeacherNamesResponse, error) {
	names, err := t.scheduleService.GetAllTeacherNames(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get teacher names")
	}

	return &pb.TeacherNamesResponse{
		Names: names,
	}, nil
}

func (t *TeacherScheduleTransport) PublishTeacherScheduleUpdate(teacher *pb.Teacher) error {
	log := t.log.With(
		"operation", "transport.teacher_schedule.TeacherScheduleTransport.PublishTeacherScheduleUpdate",
		"name", teacher.GetName(),
		"week", teacher.GetWeek().String(),
	)

	log.Info("Publishing teacher schedule update")
	resp := &pb.TeacherScheduleResponse{Teacher: teacher}

	data, err := proto.Marshal(resp)
	if err != nil {
		log.Error("failed marshalling proto", "error", err)
		return err
	}

	if err := t.natsConn.Publish("teacher_schedule.updates", data); err != nil {
		log.Error("failed publishing teacher schedule update to nats", "error", err)
		return err
	}

	return nil
}

func (t *TeacherScheduleTransport) PublishTeacherWeekUpdates(date time.Time) error {
	log := t.log.With(
		"operation", "transport.teacher_schedule.TeacherScheduleTransport.PublishTeacherWeekUpdates",
		"date", date.Format("2006-01-02"),
	)

	log.Info("publishing updated teacher week date")

	date = time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)

	data := []byte(date.Format(time.RFC3339))

	if err := t.natsConn.Publish("teacher_schedule.week.updates", data); err != nil {
		log.Error("failed to publish updated teacher week date", "error", err)
		return err
	}

	return nil
}
