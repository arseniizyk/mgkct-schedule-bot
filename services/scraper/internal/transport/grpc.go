package server

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule"
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
	sch, err := s.scheduleUC.GetLatest()
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "can't get schedule")
	}

	group, ok := sch.Groups[int(req.GroupNum)]
	if !ok {
		return nil, status.Error(codes.NotFound, "group not found")
	}
	return &pb.GroupScheduleResponse{
		Week: group.Week,
		Day:  daysToProto(group.Days),
	}, nil
}
