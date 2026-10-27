package notifier

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
)

func TestCalculateTomorrowDate(t *testing.T) {
	minskLoc := time.FixedZone("MSK", 3*3600)

	tests := []struct {
		name       string
		now        time.Time
		wantDate   string
		wantDayIdx int
	}{
		{
			name:       "Monday -> Tuesday (idx 1)",
			now:        time.Date(2026, 10, 12, 20, 0, 0, 0, minskLoc),
			wantDate:   "13.10.2026",
			wantDayIdx: 1,
		},
		{
			name:       "Tuesday -> Wednesday (idx 2)",
			now:        time.Date(2026, 10, 13, 20, 0, 0, 0, minskLoc),
			wantDate:   "14.10.2026",
			wantDayIdx: 2,
		},
		{
			name:       "Wednesday -> Thursday (idx 3)",
			now:        time.Date(2026, 10, 14, 20, 0, 0, 0, minskLoc),
			wantDate:   "15.10.2026",
			wantDayIdx: 3,
		},
		{
			name:       "Thursday -> Friday (idx 4)",
			now:        time.Date(2026, 10, 15, 20, 0, 0, 0, minskLoc),
			wantDate:   "16.10.2026",
			wantDayIdx: 4,
		},
		{
			name:       "Friday -> Saturday (idx 5)",
			now:        time.Date(2026, 10, 16, 20, 0, 0, 0, minskLoc),
			wantDate:   "17.10.2026",
			wantDayIdx: 5,
		},
		{
			name:       "Saturday -> Monday (idx 0)",
			now:        time.Date(2026, 10, 17, 20, 0, 0, 0, minskLoc),
			wantDate:   "19.10.2026",
			wantDayIdx: 0,
		},
		{
			name:       "Sunday -> Monday (idx 0)",
			now:        time.Date(2026, 10, 18, 20, 0, 0, 0, minskLoc),
			wantDate:   "19.10.2026",
			wantDayIdx: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDate, gotDayIdx := CalculateTomorrowDate(tt.now)
			if gotDate.Format("02.01.2006") != tt.wantDate {
				t.Errorf("gotDate = %s, want %s", gotDate.Format("02.01.2006"), tt.wantDate)
			}
			if gotDayIdx != tt.wantDayIdx {
				t.Errorf("gotDayIdx = %d, want %d", gotDayIdx, tt.wantDayIdx)
			}
		})
	}
}

func TestNotifier_ScraperAlerts_NoAdmin(t *testing.T) {
	n := &Notifier{
		adminChatID: 0,
	}

	ctx := context.Background()
	if err := n.HandleScraperError(ctx, nil, 5); err != nil {
		t.Errorf("expected nil error when adminChatID=0, got %v", err)
	}
	if err := n.HandleScraperRecovered(ctx); err != nil {
		t.Errorf("expected nil error when adminChatID=0, got %v", err)
	}
}

func TestIsSameWeek(t *testing.T) {
	minskLoc := time.FixedZone("MSK", 3*3600)
	monday := time.Date(2026, 10, 12, 10, 0, 0, 0, minskLoc)
	wednesday := time.Date(2026, 10, 14, 15, 0, 0, 0, minskLoc)
	nextMonday := time.Date(2026, 10, 19, 10, 0, 0, 0, minskLoc)

	if !isSameWeek(monday, wednesday) {
		t.Errorf("expected monday and wednesday of the same week to be same week")
	}
	if isSameWeek(monday, nextMonday) {
		t.Errorf("expected monday and next monday to be different weeks")
	}
}

type mockSchedProvider struct {
	byWeekFunc func(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error)
	latestFunc func(ctx context.Context, groupID int32) (*entities.Group, error)
}

func (m *mockSchedProvider) GetGroupScheduleByWeek(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
	if m.byWeekFunc != nil {
		return m.byWeekFunc(ctx, groupID, week)
	}
	return nil, nil
}

func (m *mockSchedProvider) GetGroupLatestSchedule(ctx context.Context, groupID int32) (*entities.Group, error) {
	if m.latestFunc != nil {
		return m.latestFunc(ctx, groupID)
	}
	return nil, nil
}

type mockTeacherProvider struct {
	byWeekFunc func(ctx context.Context, name string, week time.Time) (*entities.Teacher, error)
	latestFunc func(ctx context.Context, name string) (*entities.Teacher, error)
	memberFunc func(ctx context.Context, teacherName string) (*entities.FacultyMember, error)
}

func (m *mockTeacherProvider) GetTeacherScheduleByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	if m.byWeekFunc != nil {
		return m.byWeekFunc(ctx, name, week)
	}
	return nil, nil
}

func (m *mockTeacherProvider) GetTeacherLatestSchedule(ctx context.Context, name string) (*entities.Teacher, error) {
	if m.latestFunc != nil {
		return m.latestFunc(ctx, name)
	}
	return nil, nil
}

func (m *mockTeacherProvider) GetFacultyMember(ctx context.Context, teacherName string) (*entities.FacultyMember, error) {
	if m.memberFunc != nil {
		return m.memberFunc(ctx, teacherName)
	}
	return nil, nil
}

func TestFormatNextDayDigest_NeitherGroupNorTeacher(t *testing.T) {
	ctx := context.Background()
	_, err := FormatNextDayDigest(ctx, entities.User{}, time.Now(), 0, nil, nil)
	if err == nil {
		t.Fatalf("expected error when neither group nor teacher specified")
	}
}

func TestFormatNextDayDigest_GroupSuccess(t *testing.T) {
	ctx := context.Background()
	minskLoc := time.FixedZone("MSK", 3*3600)
	targetDate := time.Date(2026, 10, 13, 12, 0, 0, 0, minskLoc) // Tuesday
	targetMonday := time.Date(2026, 10, 12, 0, 0, 0, 0, minskLoc)

	schedProv := &mockSchedProvider{
		byWeekFunc: func(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
			return &entities.Group{
				ID:   groupID,
				Week: targetMonday,
				Days: []*entities.Day{
					{Name: "Понедельник"},
					{
						Name: "Вторник",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "Математика", Class: "101"}}},
						},
					},
				},
			}, nil
		},
	}

	user := entities.User{ChatID: 12345, Group: 101}
	msg, err := FormatNextDayDigest(ctx, user, targetDate, 1, schedProv, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "Расписание на") || !strings.Contains(msg, "13.10.2026") {
		t.Errorf("expected header with date, got %s", msg)
	}
	if !strings.Contains(msg, "Математика") {
		t.Errorf("expected subject in message, got %s", msg)
	}
}

func TestFormatNextDayDigest_GroupSuccess(t *testing.T) {
	ctx := context.Background()
	minskLoc := time.FixedZone("MSK", 3*3600)
	targetDate := time.Date(2026, 10, 13, 12, 0, 0, 0, minskLoc) // Tuesday
	targetMonday := time.Date(2026, 10, 12, 0, 0, 0, 0, minskLoc)

	schedProv := &mockSchedProvider{
		byWeekFunc: func(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
			return &entities.Group{
				ID:   groupID,
				Week: targetMonday,
				Days: []*entities.Day{
					{Name: "Понедельник"},
					{
						Name: "Вторник",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "Математика", Class: "101"}}},
						},
					},
				},
			}, nil
		},
	}

	user := entities.User{ChatID: 12345, Group: 101}
	msg, err := FormatNextDayDigest(ctx, user, targetDate, 1, schedProv, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "Расписание на") || !strings.Contains(msg, "13.10.2026") {
		t.Errorf("expected header with date, got %s", msg)
	}
	if !strings.Contains(msg, "Математика") {
		t.Errorf("expected subject in message, got %s", msg)
	}
}

func TestFormatNextDayDigest_TeacherSuccess(t *testing.T) {
	ctx := context.Background()
	minskLoc := time.FixedZone("MSK", 3*3600)
	targetDate := time.Date(2026, 10, 14, 12, 0, 0, 0, minskLoc) // Wednesday
	targetMonday := time.Date(2026, 10, 12, 0, 0, 0, 0, minskLoc)

	teacherProv := &mockTeacherProvider{
		byWeekFunc: func(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
			return &entities.Teacher{
				Name: name,
				Week: targetMonday,
				Days: []*entities.Day{
					{Name: "Понедельник"},
					{Name: "Вторник"},
					{
						Name: "Среда",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "Физика", Group: "101", Class: "202"}}},
						},
					},
				},
			}, nil
		},
		memberFunc: func(ctx context.Context, teacherName string) (*entities.FacultyMember, error) {
			return &entities.FacultyMember{FullName: teacherName, Department: "Кафедра физики"}, nil
		},
	}

	user := entities.User{ChatID: 12345, TeacherName: "Иванов И. И."}
	msg, err := FormatNextDayDigest(ctx, user, targetDate, 2, nil, teacherProv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "Расписание на") || !strings.Contains(msg, "14.10.2026") {
		t.Errorf("expected header with date, got %s", msg)
	}
	if !strings.Contains(msg, "Физика") {
		t.Errorf("expected subject in message, got %s", msg)
	}
}
func TestFormatNextDayDigest_TeacherSuccess(t *testing.T) {
	ctx := context.Background()
	minskLoc := time.FixedZone("MSK", 3*3600)
	targetDate := time.Date(2026, 10, 14, 12, 0, 0, 0, minskLoc) // Wednesday
	targetMonday := time.Date(2026, 10, 12, 0, 0, 0, 0, minskLoc)

	teacherProv := &mockTeacherProvider{
		byWeekFunc: func(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
			return &entities.Teacher{
				Name: name,
				Week: targetMonday,
				Days: []*entities.Day{
					{Name: "Понедельник"},
					{Name: "Вторник"},
					{
						Name: "Среда",
						Subjects: []*entities.Subject{
							{Pairs: []*entities.Pair{{Name: "Физика", Group: "101", Class: "202"}}},
						},
					},
				},
			}, nil
		},
		memberFunc: func(ctx context.Context, teacherName string) (*entities.FacultyMember, error) {
			return &entities.FacultyMember{FullName: teacherName, Department: "Кафедра физики"}, nil
		},
	}

	user := entities.User{ChatID: 12345, TeacherName: "Иванов И. И."}
	msg, err := FormatNextDayDigest(ctx, user, targetDate, 2, nil, teacherProv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(msg, "Расписание на") || !strings.Contains(msg, "14.10.2026") {
		t.Errorf("expected header with date, got %s", msg)
	}
	if !strings.Contains(msg, "Физика") {
		t.Errorf("expected subject in message, got %s", msg)
	}
}


func TestFormatNextDayDigest_WeekNotPublishedYet(t *testing.T) {
	ctx := context.Background()
	minskLoc := time.FixedZone("MSK", 3*3600)
	targetDate := time.Date(2026, 10, 19, 12, 0, 0, 0, minskLoc) // Next Monday
	oldMonday := time.Date(2026, 10, 12, 0, 0, 0, 0, minskLoc)   // Current Monday

	schedProv := &mockSchedProvider{
		byWeekFunc: func(ctx context.Context, groupID int32, week time.Time) (*entities.Group, error) {
			return nil, errNotFound
		},
		latestFunc: func(ctx context.Context, groupID int32) (*entities.Group, error) {
			return &entities.Group{
				ID:   groupID,
				Week: oldMonday, // old week!
				Days: []*entities.Day{
					{Name: "Понедельник"},
				},
			}, nil
		},
	}

	user := entities.User{ChatID: 12345, Group: 101}
	_, err := FormatNextDayDigest(ctx, user, targetDate, 0, schedProv, nil)
	if err == nil {
		t.Fatalf("expected error when next week is not published yet, got nil")
	}
	if !strings.Contains(err.Error(), "not published yet") {
		t.Errorf("expected 'not published yet' error, got %v", err)
	}
}

var errNotFound = &notFoundError{}

type notFoundError struct{}

func (e *notFoundError) Error() string { return "not found" }
