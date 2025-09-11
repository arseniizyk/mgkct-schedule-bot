package server

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	scheduleUC "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/schedule/usecase"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	scheduleUC *scheduleUC.ScheduleUsecase
	pb.UnimplementedScheduleServiceServer
}

func NewGRPCServer(schUC *scheduleUC.ScheduleUsecase) *GRPCServer {
	return &GRPCServer{scheduleUC: schUC}
}

func (s *GRPCServer) GetGroupSchedule(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	sch, err := s.scheduleUC.GetLatest()
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "can't get schedule")
	}

	group := sch.Groups[int(req.GroupNum)]
	return &pb.GroupScheduleResponse{
		Week: group.Week,
		Day:  fillDays(group.Days),
	}, nil
}
