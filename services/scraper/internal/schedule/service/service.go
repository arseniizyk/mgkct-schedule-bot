package service

import (
	"context"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/model"
)

type ScheduleService interface {
	GetFullLatestSchedule(ctx context.Context) (*pb.Schedule, error)
	GetGroupLatestSchedule(ctx context.Context, groupID int32) (*pb.Group, error)
	GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*pb.Group, error)
	CheckScheduleUpdates(interval time.Duration) <-chan *model.Updated
}
