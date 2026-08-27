package app

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

type grpcAdapter struct {
	pb.UnimplementedScheduleServiceServer

	scheduleTransport        ScheduleTransport
	weekTransport            WeekTransport
	teacherScheduleTransport TeacherScheduleTransport
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

func (g *grpcAdapter) GetTeacherSchedule(ctx context.Context, req *pb.TeacherScheduleRequest) (*pb.TeacherScheduleResponse, error) {
	return g.teacherScheduleTransport.GetTeacherSchedule(ctx, req)
}

func (g *grpcAdapter) GetTeacherScheduleByWeek(ctx context.Context, req *pb.TeacherScheduleRequest) (*pb.TeacherScheduleResponse, error) {
	return g.teacherScheduleTransport.GetTeacherScheduleByWeek(ctx, req)
}

func (g *grpcAdapter) GetAvailableTeacherWeeks(ctx context.Context, req *pb.AvailableWeeksRequest) (*pb.AvailableWeeksResponse, error) {
	return g.teacherScheduleTransport.GetAvailableTeacherWeeks(ctx, req)
}

func (g *grpcAdapter) GetTeacherNames(ctx context.Context, req *pb.Empty) (*pb.TeacherNamesResponse, error) {
	return g.teacherScheduleTransport.GetTeacherNames(ctx, req)
}
