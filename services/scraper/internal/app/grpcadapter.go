package app

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/transport"
)

type grpcAdapter struct {
	transport transport.Schedule
	pb.UnimplementedScheduleServiceServer
}

func (g *grpcAdapter) GetGroupSchedule(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	return g.transport.GetGroupSchedule(ctx, req)
}

func (g *grpcAdapter) GetGroupScheduleByWeek(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	return g.transport.GetGroupScheduleByWeek(ctx, req)
}

func (g *grpcAdapter) GetAvailableWeeks(ctx context.Context, req *pb.AvailableWeeksRequest) (*pb.AvailableWeeksResponse, error) {
	return g.transport.GetAvailableWeeks(ctx, req)
}
