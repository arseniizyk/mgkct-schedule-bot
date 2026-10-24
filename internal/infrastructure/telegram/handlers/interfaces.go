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
	GetAllWeeks(ctx context.Context) ([]time.Time, error)
	GetArchiveSchedule(ctx context.Context, groupID int32, targetDate time.Time) (*entities.Group, error)
}

type TeacherService interface {
	GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error)
	GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error)
	GetAvailableWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error)
	GetAllTeacherNames(ctx context.Context) ([]string, error)
	GetAllWeeks(ctx context.Context, name string) ([]time.Time, error)
	GetTeacherArchiveSchedule(ctx context.Context, name string, targetDate time.Time) (*entities.Teacher, error)
	ValidateTeacher(ctx context.Context, name string) (string, []string, bool)
	FindSimilarTeachers(ctx context.Context, name string, limit int) ([]string, error)
	GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error)
}

type UserRepo interface {
	SaveUser(ctx context.Context, u entities.User) error
	GroupByChatID(ctx context.Context, chatID int64) (int, error)
	SubgroupByChatID(ctx context.Context, chatID int64) (int, error)
	GroupAndSubgroupByChatID(ctx context.Context, chatID int64) (int, int, error)
	SetUserGroup(ctx context.Context, chatID int64, groupID int, subgroup int) error
	GetTeacher(ctx context.Context, chatID int64) (string, error)
	SetTeacher(ctx context.Context, chatID int64, teacherName string) error
	SetNotifyEvening(ctx context.Context, chatID int64, enabled bool) error
	GetNotifyEvening(ctx context.Context, chatID int64) (bool, error)
}

type StateManager interface {
	Get(chatID int64) (state.State, bool)
	Set(chatID int64, st state.State) error
	Clear(chatID int64) error
}

type AnalyticsService interface {
	GetAllCabinets(ctx context.Context) ([]string, error)
	GetCabinetSchedule(ctx context.Context, query string, week time.Time) (*entities.CabinetSchedule, []string, error)
	GetEndings(ctx context.Context, dayIdx int, week time.Time) (*entities.EndingsStat, error)
	GetGroupStats(ctx context.Context, groupID int32, week time.Time) ([]entities.SubjectStat, int, error)
	GetTeacherStats(ctx context.Context, teacherName string, week time.Time) ([]entities.SubjectStat, int, error)
}
