package server

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	sch *models.Schedule
	pb.UnimplementedScheduleServiceServer
}

func NewGRPCServer(schedule *models.Schedule) *GRPCServer {
	return &GRPCServer{sch: schedule}
}

func (s *GRPCServer) GetGroupSchedule(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	group, ok := s.sch.Groups[int(req.GroupNum)]
	if !ok {
		return nil, status.Errorf(codes.NotFound, "group %d not found", req.GroupNum)
	}

	return &pb.GroupScheduleResponse{
		Week: group.Week,
		Day:  fillDays(group.Days),
	}, nil
}
