package app

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

type grpcAdapter struct {
	scheduleTransport ScheduleTransport
	weekTransport     WeekTransport
	pb.UnimplementedScheduleServiceServer
}

func (g *grpcAdapter) GetGroupSchedule(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	return g.scheduleTransport.GetGroupSchedule(ctx, req)
}

func (g *grpcAdapter) GetGroupScheduleByWeek(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	return g.scheduleTransport.GetGroupScheduleByWeek(ctx, req)
}

func (g *grpcAdapter) GetAvailableWeeks(ctx context.Context, req *pb.AvailableWeeksRequest) (*pb.AvailableWeeksResponse, error) {
	return g.weekTransport.GetAvailableWeeks(ctx, req)
}
