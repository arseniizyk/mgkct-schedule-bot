package transport

import (
	"context"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

type ScheduleTransport interface {
	GetGroupSchedule(ctx context.Context, groupNum int) (*pb.Group, error)
	GetGroupScheduleByWeek(ctx context.Context, groupNum int, week time.Time) (*pb.Group, error)
	SubscribeScheduleUpdates() error
}
