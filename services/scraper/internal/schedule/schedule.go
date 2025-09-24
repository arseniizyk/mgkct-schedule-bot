package schedule

import (
	"context"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

type ScheduleUsecase interface {
	GetLatest() (*pb.Schedule, error)
	SaveToCache(sch *pb.Schedule)
	Save(ctx context.Context, week time.Time, sch *pb.Schedule) error
}
