package usecases

import (
	"context"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
)

type WeekUsecase interface {
	GetAvailableWeeks(ctx context.Context, week *time.Time) (entities.Weeks, error)
}

type ScheduleUsecase interface {
	GetGroupScheduleByChatID(ctx context.Context, chatID int64) (*pb.Group, error)
	GetGroupSchedule(ctx context.Context, groupID int) (*pb.Group, error)
	GetGroupScheduleByWeek(ctx context.Context, groupID int, week time.Time) (*pb.Group, error)
}
