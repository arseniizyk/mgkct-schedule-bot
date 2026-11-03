package handlers_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/handlers"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/keyboard"
)

type mockCalendarScheduleService struct {
	group *entities.Group
	weeks entities.WeekNavigation
}

func (m *mockCalendarScheduleService) GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error) {
	return m.group, nil
}
func (m *mockCalendarScheduleService) GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
	return m.group, nil
}
func (m *mockCalendarScheduleService) GetAvailableWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error) {
	return m.weeks, nil
}
func (m *mockCalendarScheduleService) GetAllWeeks(ctx context.Context) ([]time.Time, error) {
	return nil, nil
}
func (m *mockCalendarScheduleService) GetArchiveSchedule(ctx context.Context, groupID int32, targetDate time.Time) (*entities.Group, error) {
	return m.group, nil
}
func (m *mockCalendarScheduleService) HasGroup(ctx context.Context, groupID int32) (bool, error) {
	return true, nil
}

type mockCalendarTeacherService struct {
	teacher *entities.Teacher
	weeks   entities.WeekNavigation
}

func (m *mockCalendarTeacherService) GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error) {
	return m.teacher, nil
}
func (m *mockCalendarTeacherService) GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	return m.teacher, nil
}
func (m *mockCalendarTeacherService) GetAvailableWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error) {
	return m.weeks, nil
}
func (m *mockCalendarTeacherService) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	if m.teacher == nil {
		return nil, nil
	}
	return []string{m.teacher.Name}, nil
}
func (m *mockCalendarTeacherService) GetAllWeeks(ctx context.Context, name string) ([]time.Time, error) {
	return nil, nil
}
func (m *mockCalendarTeacherService) GetTeacherArchiveSchedule(ctx context.Context, name string, targetDate time.Time) (*entities.Teacher, error) {
	return m.teacher, nil
}
func (m *mockCalendarTeacherService) ValidateTeacher(ctx context.Context, name string) (string, []string, bool) {
	if m.teacher == nil {
		return "", nil, false
	}
	return m.teacher.Name, nil, true
}
func (m *mockCalendarTeacherService) FindSimilarTeachers(ctx context.Context, name string, limit int) ([]string, error) {
	return nil, nil
}
func (m *mockCalendarTeacherService) GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error) {
	return nil, nil
}

type mockCalendarTeleContext struct {
	tele.Context

	chat      *tele.Chat
	callback  *tele.Callback
	sentWhat  any
	sentOpts  []any
	responded bool
	respText  string
	args      []string
}

func (m *mockCalendarTeleContext) Chat() *tele.Chat {
	if m.chat != nil {
		return m.chat
	}
	return &tele.Chat{ID: 12345}
}

func (m *mockCalendarTeleContext) Recipient() tele.Recipient {
	return m.Chat()
}

func (m *mockCalendarTeleContext) Args() []string {
	return m.args
}

func (m *mockCalendarTeleContext) Callback() *tele.Callback {
	return m.callback
}

func (m *mockCalendarTeleContext) Data() string {
	if m.callback != nil {
		return m.callback.Data
	}
	return ""
}

func (m *mockCalendarTeleContext) Send(what any, opts ...any) error {
	m.sentWhat = what
	m.sentOpts = opts
	return nil
}

func (m *mockCalendarTeleContext) Respond(resp ...*tele.CallbackResponse) error {
	m.responded = true
	if len(resp) > 0 && resp[0] != nil {
		m.respText = resp[0].Text
	}
	return nil
}

func TestScheduleCalendarHandler(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	monday := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)

	group := &entities.Group{
		ID:   80,
		Week: monday,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Математика", Type: "Лек", Class: "101"},
						},
					},
				},
			},
		},
	}

	schedSvc := &mockCalendarScheduleService{
		group: group,
		weeks: entities.WeekNavigation{
			Current: monday,
		},
	}

	teacher := &entities.Teacher{
		Name: "Иванов И. И.",
		Week: monday,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Физика", Group: "80", Class: "202"},
						},
					},
				},
			},
		},
	}

	teacherSvc := &mockCalendarTeacherService{
		teacher: teacher,
		weeks: entities.WeekNavigation{
			Current: monday,
		},
	}

	userRepo := newMockUserRepo()
	userRepo.userGroup[12345] = 80
	userRepo.userSubgroup[12345] = 1

	handler := handlers.ScheduleCalendar(log, schedSvc, teacherSvc, userRepo, nil)

	t.Run("export by explicit group argument without next week sends file directly", func(t *testing.T) {
		c := &mockCalendarTeleContext{args: []string{"80", "1"}}
		err := handler(c)
		require.NoError(t, err)

		doc, ok := c.sentWhat.(*tele.Document)
		require.True(t, ok, "expected *tele.Document sent")
		assert.Equal(t, "text/calendar", doc.MIME)
		assert.Contains(t, doc.FileName, "Расписание_Группа_80")
		assert.Contains(t, doc.Caption, "Расписание группы 80")

		require.NotNil(t, doc.File.FileReader)
		bytes, err := io.ReadAll(doc.File.FileReader)
		require.NoError(t, err)
		assert.Contains(t, string(bytes), "BEGIN:VCALENDAR")
		assert.Contains(t, string(bytes), "SUMMARY:Математика (Лек)")
	})

	t.Run("export with multiple weeks prompts choice keyboard", func(t *testing.T) {
		schedSvcMulti := &mockCalendarScheduleService{
			group: group,
			weeks: entities.WeekNavigation{
				Current: monday,
				Next:    monday.AddDate(0, 0, 7),
			},
		}

		hMulti := handlers.ScheduleCalendar(log, schedSvcMulti, teacherSvc, userRepo, nil)
		c := &mockCalendarTeleContext{args: []string{"80"}}
		err := hMulti(c)
		require.NoError(t, err)

		text, ok := c.sentWhat.(string)
		require.True(t, ok, "expected string message with markup")
		assert.Contains(t, text, "Выберите неделю для выгрузки")
		require.NotEmpty(t, c.sentOpts)
	})

	t.Run("export by explicit teacher argument sends file directly", func(t *testing.T) {
		c := &mockCalendarTeleContext{args: []string{"Иванов И. И."}}
		err := handler(c)
		require.NoError(t, err)

		doc, ok := c.sentWhat.(*tele.Document)
		require.True(t, ok, "expected *tele.Document sent")
		assert.Equal(t, "text/calendar", doc.MIME)
		assert.Contains(t, doc.FileName, "Расписание_Иванов_И._И.")
	})
}

func TestHandleCalendarCallbacks(t *testing.T) {
	log := slog.New(slog.DiscardHandler)
	monday := time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC)

	group := &entities.Group{
		ID:   80,
		Week: monday,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "Информатика", Type: "Лаб", Class: "101"},
						},
					},
				},
			},
		},
	}

	schedSvc := &mockCalendarScheduleService{
		group: group,
	}

	teacher := &entities.Teacher{
		Name: "Иванов И. И.",
		Week: monday,
		Days: []*entities.Day{
			{
				Name: "Понедельник",
				Subjects: []*entities.Subject{
					{
						Pairs: []*entities.Pair{
							{Name: "ТРПО", Class: "303"},
						},
					},
				},
			},
		},
	}

	teacherSvc := &mockCalendarTeacherService{
		teacher: teacher,
	}

	userRepo := newMockUserRepo()
	userRepo.userGroup[12345] = 80
	userRepo.userSubgroup[12345] = 1

	cbHandler := handlers.CallbacksHandler(log, schedSvc, teacherSvc, nil, userRepo, nil)

	t.Run("group calendar callback", func(t *testing.T) {
		c := &mockCalendarTeleContext{
			callback: &tele.Callback{
				Data: fmt.Sprintf("\f%s|80:09.11.2026:1", keyboard.WeekCalendar),
			},
		}

		err := cbHandler(c)
		require.NoError(t, err)
		assert.True(t, c.responded)

		doc, ok := c.sentWhat.(*tele.Document)
		require.True(t, ok, "expected *tele.Document sent")
		assert.Equal(t, "text/calendar", doc.MIME)
		assert.Contains(t, doc.FileName, "Расписание_Группа_80_09.11.2026.ics")
	})

	t.Run("teacher calendar callback", func(t *testing.T) {
		c := &mockCalendarTeleContext{
			callback: &tele.Callback{
				Data: fmt.Sprintf("\f%s|Иванов И. И.:09.11.2026", keyboard.TeacherWeekCalendar),
			},
		}

		err := cbHandler(c)
		require.NoError(t, err)
		assert.True(t, c.responded)

		doc, ok := c.sentWhat.(*tele.Document)
		require.True(t, ok, "expected *tele.Document sent")
		assert.Equal(t, "text/calendar", doc.MIME)
		assert.Contains(t, doc.FileName, "Расписание_Иванов_И._И._09.11.2026.ics")
	})
}
