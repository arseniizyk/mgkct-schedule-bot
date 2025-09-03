package grpc

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/crawler"
)

type Server struct {
	sch crawler.Schedule
	pb.UnimplementedScheduleServiceServer
}

func New(schedule crawler.Schedule) *Server {
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

func fillDays(days []crawler.Day) []*pb.Day {
	res := make([]*pb.Day, len(days))

	for i, d := range days {
		pbDay := &pb.Day{
			Name:    d.Name,
			Subject: fillSubjects(d.Subjects),
		}
		res[i] = pbDay
	}

	return res
}

func fillSubjects(subjects []crawler.Subject) []*pb.Subject {
	res := make([]*pb.Subject, len(subjects))

	for i, s := range subjects {
		res[i] = &pb.Subject{
			Name:  s.Name,
			Class: s.Class,
			Empty: s.IsEmpty,
		}
	}

	return res
}
