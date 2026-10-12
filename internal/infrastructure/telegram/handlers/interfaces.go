package handlers

import (
	"context"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/state"
)

type ScheduleService interface {
	GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error)
	GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error)
	GetAvailableWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error)
}

type TeacherService interface {
	GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error)
	GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error)
	GetAvailableWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error)
	GetAllTeacherNames(ctx context.Context) ([]string, error)
	ValidateTeacher(ctx context.Context, name string) (string, []string, bool)
	GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error)
}

type UserRepo interface {
	SaveUser(ctx context.Context, u entities.User) error
	GroupByChatID(ctx context.Context, chatID int64) (int, error)
	SetUserGroup(ctx context.Context, chatID int64, groupID int) error
	GetTeacher(ctx context.Context, chatID int64) (string, error)
	SetTeacher(ctx context.Context, chatID int64, teacherName string) error
}

type StateManager interface {
	Get(chatID int64) (state.State, bool)
	Set(chatID int64, st state.State) error
	Clear(chatID int64) error
}
