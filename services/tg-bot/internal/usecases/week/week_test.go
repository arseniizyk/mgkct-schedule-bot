package week

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
)

type mockWeekTransport struct {
	weeks entities.Weeks
	err   error

	gotNil bool
	last   *time.Time
}

func (m *mockWeekTransport) GetAvailableWeeks(_ context.Context, week *time.Time) (entities.Weeks, error) {
	m.gotNil = week == nil
	m.last = week
	return m.weeks, m.err
}

func TestWeekUsecase_GetAvailableWeeks(t *testing.T) {
	t.Run("nil week", func(t *testing.T) {
		tr := &mockWeekTransport{}
		uc := New(slog.Default(), tr)

		got, err := uc.GetAvailableWeeks(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !tr.gotNil {
			t.Error("transport received non-nil week, want nil")
		}
		_ = got
	})

	t.Run("week passed through", func(t *testing.T) {
		tr := &mockWeekTransport{}
		uc := New(slog.Default(), tr)

		week := time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC)
		if _, err := uc.GetAvailableWeeks(context.Background(), &week); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tr.gotNil {
			t.Error("transport received nil week")
		}
		if tr.last == nil || !tr.last.Equal(week) {
			t.Errorf("transport received %v, want %v", tr.last, week)
		}
	})

	t.Run("error passthrough", func(t *testing.T) {
		sentinel := errors.New("no weeks")
		tr := &mockWeekTransport{err: sentinel}
		uc := New(slog.Default(), tr)

		if _, err := uc.GetAvailableWeeks(context.Background(), nil); !errors.Is(err, sentinel) {
			t.Fatalf("error = %v, want %v", err, sentinel)
		}
	})
}
