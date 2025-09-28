package transport

import (
	"context"
	"errors"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/repository/postgres"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	scheduleUC schedule.ScheduleUsecase
	pb.UnimplementedScheduleServiceServer
}

func NewGRPCServer(schUC schedule.ScheduleUsecase) *GRPCServer {
	return &GRPCServer{scheduleUC: schUC}
}

func (s *GRPCServer) GetGroupSchedule(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	sch, err := s.scheduleUC.GetFullLatestSchedule(ctx)
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

func (s *GRPCServer) GetGroupScheduleByWeek(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	group, err := s.scheduleUC.GetGroupScheduleByWeek(ctx, req.Id, req.Week.AsTime())
	if err != nil {
		if errors.Is(err, postgres.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "group not found")
		}
		return nil, status.Errorf(codes.Unavailable, "can't get schedule")
	}

	return &pb.GroupScheduleResponse{
		Group: group,
	}, nil
}
