package server

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/crawler"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	sch *crawler.Schedule
	pb.UnimplementedScheduleServiceServer
}

func NewGRPCServer(schedule *crawler.Schedule) *GRPCServer {
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
