package notifier

import (
	"context"
	"time"

	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

type UserStore interface {
	AllUsers(ctx context.Context) ([]entities.User, error)
	UserIDsByGroupID(ctx context.Context, groupID int) ([]int64, error)
	UserIDsByTeacherName(ctx context.Context, teacherName string) ([]int64, error)
	UsersForEveningNotification(ctx context.Context) ([]entities.User, error)
	SetNotifyEvening(ctx context.Context, chatID int64, enabled bool) error
}

type Limiter interface {
	Wait(ctx context.Context) error
}

type ScheduleProvider interface {
	GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error)
	GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error)
}

type TeacherProvider interface {
	GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error)
	GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error)
	GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error)
}

type BotSender interface {
	Send(to tele.Recipient, what any, opts ...any) (*tele.Message, error)
}
