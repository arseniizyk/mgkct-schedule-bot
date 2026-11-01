package app

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockActivityUpdater struct {
	mu      sync.Mutex
	updated []int64
}

func (m *mockActivityUpdater) UpdateActivity(ctx context.Context, chatID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updated = append(m.updated, chatID)
	return nil
}

func (m *mockActivityUpdater) getUpdated() []int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	res := make([]int64, len(m.updated))
	copy(res, m.updated)
	return res
}

func TestActivityWorker_TouchAndDebounce(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	updater := &mockActivityUpdater{}
	worker := newActivityWorker(logger, updater)

	// 1. Первый вызов Touch добавляет chatID в очередь
	worker.Touch(12345)
	assert.Len(t, worker.ch, 1)

	// 2. Повторный вызов Touch сразу же отсекается 5-минутным дебаунсом
	worker.Touch(12345)
	assert.Len(t, worker.ch, 1)

	// 3. Другой пользователь проходит без дебаунса
	worker.Touch(67890)
	assert.Len(t, worker.ch, 2)
}

func TestActivityWorker_QueueFull_DoesNotLockLastSeen(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	updater := &mockActivityUpdater{}
	worker := newActivityWorker(logger, updater)

	// Заполняем буфер очереди (100 элементов)
	for i := 0; i < 100; i++ {
		worker.ch <- int64(i)
	}

	// Вызов Touch для нового пользователя при переполненной очереди
	worker.Touch(99999)

	// Убеждаемся, что lastSeen не зафиксирован из-за переполнения
	worker.mu.Lock()
	_, exists := worker.lastSeen[99999]
	worker.mu.Unlock()
	assert.False(t, exists, "lastSeen should not be updated when queue is full")

	// Освобождаем место в очереди
	<-worker.ch

	// Повторный вызов должен успешно добавиться
	worker.Touch(99999)
	worker.mu.Lock()
	_, exists = worker.lastSeen[99999]
	worker.mu.Unlock()
	assert.True(t, exists, "lastSeen should be updated after space freed")
}

func TestActivityWorker_Run_ProcessesAndStopsGracefully(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	updater := &mockActivityUpdater{}
	worker := newActivityWorker(logger, updater)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	worker.Touch(111)
	worker.Touch(222)

	require.Eventually(t, func() bool {
		return len(updater.getUpdated()) == 2
	}, 1*time.Second, 10*time.Millisecond)

	assert.Equal(t, []int64{111, 222}, updater.getUpdated())

	// Проверяем graceful остановку по контексту
	cancel()
	select {
	case <-done:
		// Успешно завершился
	case <-time.After(1 * time.Second):
		t.Fatal("worker did not stop on ctx cancel")
	}
}
