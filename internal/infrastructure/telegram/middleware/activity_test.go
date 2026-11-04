package middleware

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockActivityUpdater struct {
	mu     sync.Mutex
	calls  []int64
	called atomic.Int32
}

func (m *mockActivityUpdater) UpdateActivity(ctx context.Context, chatID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, chatID)
	m.called.Add(1)
	return nil
}

func TestActivityWorker_TouchThrottling(t *testing.T) {
	updater := &mockActivityUpdater{}
	logger := slog.New(slog.DiscardHandler)
	worker := NewActivityWorker(logger, updater)

	worker.Touch(12345)
	assert.Len(t, worker.ch, 1)

	// Second touch within 5 minutes must be throttled
	worker.Touch(12345)
	assert.Len(t, worker.ch, 1)

	// Different chatID should not be throttled
	worker.Touch(67890)
	assert.Len(t, worker.ch, 2)
}

func TestActivityWorker_RunProcessQueue(t *testing.T) {
	updater := &mockActivityUpdater{}
	logger := slog.New(slog.DiscardHandler)
	worker := NewActivityWorker(logger, updater)

	ctx := t.Context()

	go worker.Run(ctx)

	worker.Touch(111)
	worker.Touch(222)

	require.Eventually(t, func() bool {
		return updater.called.Load() == 2
	}, 1*time.Second, 10*time.Millisecond)

	updater.mu.Lock()
	assert.ElementsMatch(t, []int64{111, 222}, updater.calls)
	updater.mu.Unlock()
}
