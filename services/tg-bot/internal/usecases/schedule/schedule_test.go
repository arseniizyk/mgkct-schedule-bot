package schedule

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/errors"
)

type mockUserRepository struct {
	group int
	err   error
}

func (m *mockUserRepository) SaveUser(_ context.Context, _ entities.User) error {
	return nil
}

func (m *mockUserRepository) AllUsers(_ context.Context) ([]entities.User, error) {
	return nil, nil
}

func (m *mockUserRepository) GroupByChatID(_ context.Context, _ int64) (int, error) {
	return m.group, m.err
}

func (m *mockUserRepository) UserIDsByGroupID(_ context.Context, _ int) ([]int64, error) {
	return nil, nil
}

func (m *mockUserRepository) SetUserGroup(_ context.Context, chatID int64, groupID int) error {
	_, _ = chatID, groupID
	return nil
}

func (m *mockUserRepository) SetTeacher(_ context.Context, chatID int64, teacherName string) error {
	_, _ = chatID, teacherName
	return nil
}

func (m *mockUserRepository) GetTeacher(_ context.Context, chatID int64) (string, error) {
	_ = chatID
	return "", nil
}

func (m *mockUserRepository) UserIDsByTeacherName(_ context.Context, teacherName string) ([]int64, error) {
	_ = teacherName
	return nil, nil
}

func (m *mockUserRepository) GetState(_ context.Context, _ int64) (string, error) {
	return "", nil
}

func (m *mockUserRepository) SetState(_ context.Context, _ int64, _ string) error {
	return nil
}

type mockScheduleTransport struct {
	group    *pb.Group
	getErr   error
	weekErr  error
	lastCall struct {
		method   string
		groupID  int
		weekTime time.Time
	}
}

func (m *mockScheduleTransport) GetGroupSchedule(_ context.Context, groupID int) (*pb.Group, error) {
	m.lastCall.method = "GetGroupSchedule"
	m.lastCall.groupID = groupID
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.group, nil
}

func (m *mockScheduleTransport) GetGroupScheduleByWeek(_ context.Context, groupID int, week time.Time) (*pb.Group, error) {
	m.lastCall.method = "GetGroupScheduleByWeek"
	m.lastCall.groupID = groupID
	m.lastCall.weekTime = week
	if m.weekErr != nil {
		return nil, m.weekErr
	}
	return m.group, nil
}

var (
	errTransportDown   = errors.New("grpc down")
	errScheduleNoWeek  = errors.New("week not found")
	errScheduleNoGroup = errors.New("not found")
)

//nolint:gochecknoglobals // общий фикстур для всех подтестов
var testGroup = &pb.Group{Id: 99}

func TestScheduleUsecase_GetGroupScheduleByChatID(t *testing.T) {
	tests := []struct {
		name       string
		repo       mockUserRepository
		transport  mockScheduleTransport
		wantErr    error
		wantCalled string
	}{
		{
			name:       "success",
			repo:       mockUserRepository{group: 99},
			transport:  mockScheduleTransport{group: testGroup},
			wantCalled: "GetGroupSchedule",
		},
		{
			name:    "user has no group",
			repo:    mockUserRepository{err: domainerr.ErrUserNoGroup},
			wantErr: domainerr.ErrUserNoGroup,
		},
		{
			name:      "transport error",
			repo:      mockUserRepository{group: 99},
			transport: mockScheduleTransport{getErr: errTransportDown},
			wantErr:   errTransportDown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := tt.transport
			uc := New(slog.Default(), &tt.repo, &tr)

			got, err := uc.GetGroupScheduleByChatID(context.Background(), 42)

			if tt.repo.err == nil && tt.transport.getErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got != testGroup {
					t.Errorf("got %v, want %v", got, testGroup)
				}
				if tr.lastCall.groupID != 99 {
					t.Errorf("called with group_id = %d, want 99", tr.lastCall.groupID)
				}
				return
			}

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got != nil {
				t.Errorf("expected nil group on error, got %v", got)
			}
		})
	}
}

func TestScheduleUsecase_GetGroupSchedule(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		tr := &mockScheduleTransport{group: testGroup}
		uc := New(slog.Default(), &mockUserRepository{}, tr)

		got, err := uc.GetGroupSchedule(context.Background(), 99)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != testGroup {
			t.Errorf("got %v, want %v", got, testGroup)
		}
	})

	t.Run("error passthrough", func(t *testing.T) {
		tr := &mockScheduleTransport{getErr: errScheduleNoGroup}
		uc := New(slog.Default(), &mockUserRepository{}, tr)

		if _, err := uc.GetGroupSchedule(context.Background(), 1); !errors.Is(err, errScheduleNoGroup) {
			t.Fatalf("error = %v, want %v", err, errScheduleNoGroup)
		}
	})
}

func TestScheduleUsecase_GetGroupScheduleByWeek(t *testing.T) {
	t.Run("success passes week to transport", func(t *testing.T) {
		tr := &mockScheduleTransport{group: testGroup}
		uc := New(slog.Default(), &mockUserRepository{}, tr)

		week := time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC)
		got, err := uc.GetGroupScheduleByWeek(context.Background(), 99, week)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != testGroup {
			t.Errorf("got %v, want %v", got, testGroup)
		}
		if !tr.lastCall.weekTime.Equal(week) {
			t.Errorf("transport received week = %v, want %v", tr.lastCall.weekTime, week)
		}
	})

	t.Run("error passthrough", func(t *testing.T) {
		tr := &mockScheduleTransport{weekErr: errScheduleNoWeek}
		uc := New(slog.Default(), &mockUserRepository{}, tr)

		if _, err := uc.GetGroupScheduleByWeek(context.Background(), 99, time.Now()); !errors.Is(err, errScheduleNoWeek) {
			t.Fatalf("error = %v, want %v", err, errScheduleNoWeek)
		}
	})
}
