package handlers

import (
	"context"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/transport/telegram/state"
)

type ScheduleProvider interface {
	GetGroupScheduleByChatID(ctx context.Context, chatID int64) (*pb.Group, error)
	GetGroupSchedule(ctx context.Context, groupID int) (*pb.Group, error)
	GetGroupScheduleByWeek(ctx context.Context, groupID int, week time.Time) (*pb.Group, error)
}

type WeekProvider interface {
	GetAvailableWeeks(ctx context.Context, week *time.Time) (entities.Weeks, error)
}

type StateGetter interface {
	Get(chatID int64) (state.State, bool)
}

type StateClearer interface {
	Clear(chatID int64) error
}

type UserSaver interface {
	SaveUser(ctx context.Context, u entities.User) error
}

type UserGroupSetter interface {
	SetUserGroup(ctx context.Context, chatID int64, groupID int) error
}

type StateSetter interface {
	Set(chatID int64, state state.State) error
}

type TeacherValidator interface {
	ValidateTeacher(ctx context.Context, name string) (matched string, candidates []string, ok bool)
}

type TeacherSaver interface {
	SetTeacher(ctx context.Context, chatID int64, teacherName string) error
}

type TeacherNamesProvider interface {
	GetAllTeacherNames(ctx context.Context) ([]string, error)
}

type TeacherScheduleProvider interface {
	GetTeacherScheduleByChatID(ctx context.Context, chatID int64) (*pb.Teacher, error)
	GetTeacherSchedule(ctx context.Context, name string) (*pb.Teacher, error)
	GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*pb.Teacher, error)
	FindTeacherByName(ctx context.Context, name string) ([]string, error)
}

type TeacherWeekProvider interface {
	GetAvailableTeacherWeeks(ctx context.Context, week *time.Time) (*pb.AvailableWeeksResponse, error)
}
