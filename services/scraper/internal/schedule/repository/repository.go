package repository

import (
	"context"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

type ScheduleRepository interface {
	Save(ctx context.Context, week time.Time, schedule *pb.Schedule) error
	GetByWeek(ctx context.Context, week time.Time) (*pb.Schedule, error)
	GetLatest(ctx context.Context) (*pb.Schedule, error)
}
