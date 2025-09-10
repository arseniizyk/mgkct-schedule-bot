package server

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/database"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GRPCServer struct {
	db database.DatabaseUseCase
	pb.UnimplementedScheduleServiceServer
}

func NewGRPCServer(db database.DatabaseUseCase) *GRPCServer {
	return &GRPCServer{db: db}
}

func (s *GRPCServer) GetGroupSchedule(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	sch, err := s.db.GetLatestSchedule(context.Background())
	if err != nil {
		return nil, status.Errorf(codes.Unavailable, "can't get schedule")
	}

	group := sch.Groups[int(req.GroupNum)]

	return &pb.GroupScheduleResponse{
		Week: group.Week,
		Day:  fillDays(group.Days),
	}, nil
}
