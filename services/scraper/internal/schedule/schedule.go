package schedule

import (
	"context"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

type ScheduleUsecase interface {
	GetFullLatestSchedule(ctx context.Context) (*pb.Schedule, error)
	GetGroupLatestSchedule(ctx context.Context, groupID int32) (*pb.Group, error)
	GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*pb.Group, error)
	SaveToCache(sch *pb.Schedule)
	Save(ctx context.Context, week time.Time, sch *pb.Schedule) error
}
