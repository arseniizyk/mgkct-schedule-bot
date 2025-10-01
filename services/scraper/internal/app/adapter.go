package app

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/transport"
)

type grpcAdapter struct {
	transport transport.ScheduleTransport
	pb.UnimplementedScheduleServiceServer
}

func (g *grpcAdapter) GetGroupSchedule(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	return g.transport.GetGroupSchedule(ctx, req)
}

func (g *grpcAdapter) GetGroupScheduleByWeek(ctx context.Context, req *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error) {
	return g.transport.GetGroupScheduleByWeek(ctx, req)
}
