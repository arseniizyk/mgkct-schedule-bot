package transport

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

type ScheduleTransport interface {
	PublishScheduleUpdate(*pb.Group) error
	GetGroupSchedule(context.Context, *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error)
	GetGroupScheduleByWeek(context.Context, *pb.GroupScheduleRequest) (*pb.GroupScheduleResponse, error)
}
