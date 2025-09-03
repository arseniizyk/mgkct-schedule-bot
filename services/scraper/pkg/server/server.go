package server

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/crawler"
)

type Server struct {
	sch *crawler.Schedule
	pb.UnimplementedScheduleServiceServer
}

func New(schedule *crawler.Schedule) *Server {
	return &Server{sch: schedule}
}

func (s *Server) GetGroupSchedule(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	group, ok := s.sch.Groups[int(req.GroupNum)]
	if !ok {
		return &pb.GroupScheduleResponse{}, nil
	}

	return &pb.GroupScheduleResponse{
		Week: group.Week,
		Day:  fillDays(group.Days),
	}, nil
}
