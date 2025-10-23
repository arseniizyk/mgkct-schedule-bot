package app

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

type grpcAdapter struct {
	transport ScheduleTransport
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
