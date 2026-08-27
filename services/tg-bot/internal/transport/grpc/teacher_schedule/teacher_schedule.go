package teacherschedule

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

	domainerr "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/errors"
)

type TeacherService interface {
	GetTeacherSchedule(ctx context.Context, req *pb.TeacherScheduleRequest, opts ...grpc.CallOption) (*pb.TeacherScheduleResponse, error)
	GetTeacherScheduleByWeek(ctx context.Context, req *pb.TeacherScheduleRequest, opts ...grpc.CallOption) (*pb.TeacherScheduleResponse, error)
	GetAvailableTeacherWeeks(ctx context.Context, req *pb.AvailableWeeksRequest, opts ...grpc.CallOption) (*pb.AvailableWeeksResponse, error)
	GetTeacherNames(ctx context.Context, req *pb.Empty, opts ...grpc.CallOption) (*pb.TeacherNamesResponse, error)
}

type TeacherScheduleTransport struct {
	log             *slog.Logger
	natsConn        *nats.Conn
	teacherSchedule TeacherService
}

func New(log *slog.Logger, natsConn *nats.Conn, teacherSchedule TeacherService) *TeacherScheduleTransport {
	return &TeacherScheduleTransport{
		log:             log,
		natsConn:        natsConn,
		teacherSchedule: teacherSchedule,
	}
}

func (t *TeacherScheduleTransport) GetTeacherSchedule(ctx context.Context, name string) (*pb.Teacher, error) {
	log := t.log.With(
		"operation", "transport.grpc.teacherschedule.TeacherScheduleTransport.GetTeacherSchedule",
		"name", name,
	)

	resp, err := t.teacherSchedule.GetTeacherSchedule(ctx, &pb.TeacherScheduleRequest{
		Name: name,
	})
	if err != nil {
		s, ok := status.FromError(err)
		if !ok {
			log.ErrorContext(ctx, "Undefined gRPC err, failed on getting status", "error", err)
			return nil, domainerr.ErrServiceInternal
		}

		switch s.Code() {
		case codes.NotFound:
			switch s.Message() {
			case "teacher schedule not found":
				log.WarnContext(ctx, "Teacher schedule not found")
				return nil, domainerr.ErrTeacherNotFound

			default:
				log.ErrorContext(ctx, "Service internal status message", "message", s.Message())
				return nil, domainerr.ErrServiceInternal
			}

		default:
			log.ErrorContext(ctx, "Unexpected gRPC status code", "status", s.Code(), "message", s.Message())
			return nil, domainerr.ErrServiceInternal
		}
	}

	if resp.GetTeacher() == nil {
		log.ErrorContext(ctx, "gRPC вернул пустого преподавателя")
		return nil, domainerr.ErrTeacherNotFound
	}

	return resp.GetTeacher(), nil
}

func (t *TeacherScheduleTransport) GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*pb.Teacher, error) {
	log := t.log.With(
		"operation", "transport.grpc.teacherschedule.TeacherScheduleTransport.GetTeacherScheduleByWeek",
		"name", name,
		"week", week.String(),
	)

	resp, err := t.teacherSchedule.GetTeacherScheduleByWeek(ctx, &pb.TeacherScheduleRequest{
		Name: name,
		Week: timestamppb.New(week),
	})
	if err != nil {
		s, ok := status.FromError(err)
		if !ok {
			log.ErrorContext(ctx, "Undefined gRPC err, failed on getting status", "error", err)
			return nil, domainerr.ErrServiceInternal
		}

		switch s.Code() {
		case codes.NotFound:
			switch s.Message() {
			case "week not found":
				log.WarnContext(ctx, "Week not found")
				return nil, domainerr.ErrWeekNotFound

			default:
				log.ErrorContext(ctx, "Service internal status message", "message", s.Message())
				return nil, domainerr.ErrServiceInternal
			}

		default:
			log.ErrorContext(ctx, "Unexpected gRPC status code", "status", s.Code(), "message", s.Message())
			return nil, domainerr.ErrServiceInternal
		}
	}

	if resp.GetTeacher() == nil {
		log.ErrorContext(ctx, "gRPC вернул пустого преподавателя")
		return nil, domainerr.ErrTeacherNotFound
	}

	return resp.GetTeacher(), nil
}

func (t *TeacherScheduleTransport) GetAvailableTeacherWeeks(ctx context.Context, week *time.Time) (*pb.AvailableWeeksResponse, error) {
	log := t.log.With(
		"operation", "transport.grpc.teacherschedule.TeacherScheduleTransport.GetAvailableTeacherWeeks",
	)

	var req *pb.AvailableWeeksRequest
	if week != nil {
		req = &pb.AvailableWeeksRequest{
			Week: timestamppb.New(*week),
		}
	} else {
		req = &pb.AvailableWeeksRequest{}
	}

	resp, err := t.teacherSchedule.GetAvailableTeacherWeeks(ctx, req)
	if err != nil {
		s, ok := status.FromError(err)
		if !ok {
			log.ErrorContext(ctx, "Undefined gRPC err, failed on getting status", "error", err)
			return nil, domainerr.ErrServiceInternal
		}

		switch s.Code() {
		case codes.NotFound:
			switch s.Message() {
			case "week not found":
				log.WarnContext(ctx, "Week not found")
				return nil, domainerr.ErrWeekNotFound

			default:
				log.ErrorContext(ctx, "Service internal status message", "message", s.Message())
				return nil, domainerr.ErrServiceInternal
			}

		default:
			log.ErrorContext(ctx, "Unexpected gRPC status code", "status", s.Code(), "message", s.Message())
			return nil, domainerr.ErrServiceInternal
		}
	}

	if resp == nil {
		return &pb.AvailableWeeksResponse{}, nil
	}

	return resp, nil
}

func (t *TeacherScheduleTransport) GetTeacherNames(ctx context.Context) ([]string, error) {
	log := t.log.With(
		"operation", "transport.grpc.teacherschedule.TeacherScheduleTransport.GetTeacherNames",
	)

	resp, err := t.teacherSchedule.GetTeacherNames(ctx, &pb.Empty{})
	if err != nil {
		s, ok := status.FromError(err)
		if !ok {
			log.ErrorContext(ctx, "Undefined gRPC err, failed on getting status", "error", err)
			return nil, domainerr.ErrServiceInternal
		}

		log.ErrorContext(ctx, "Unexpected gRPC status code", "status", s.Code(), "message", s.Message())
		return nil, domainerr.ErrServiceInternal
	}

	if resp == nil {
		return nil, domainerr.ErrServiceInternal
	}

	return resp.GetNames(), nil
}
