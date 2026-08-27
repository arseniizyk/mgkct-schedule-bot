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

type TeacherScheduleUsecase interface {
	GetTeacherScheduleByChatID(ctx context.Context, chatID int64) (*pb.Teacher, error)
	GetTeacherSchedule(ctx context.Context, name string) (*pb.Teacher, error)
	GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*pb.Teacher, error)
	GetAvailableTeacherWeeks(ctx context.Context, week *time.Time) (*pb.AvailableWeeksResponse, error)
	GetAllTeacherNames(ctx context.Context) ([]string, error)
}
