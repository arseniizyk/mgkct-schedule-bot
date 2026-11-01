package app

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type activityUpdater interface {
	UpdateActivity(ctx context.Context, chatID int64) error
}

type activityWorker struct {
	mu       sync.Mutex
	lastSeen map[int64]time.Time
	ch       chan int64
	updater  activityUpdater
	log      *slog.Logger
}

func newActivityWorker(log *slog.Logger, updater activityUpdater) *activityWorker {
	return &activityWorker{
		lastSeen: make(map[int64]time.Time),
		ch:       make(chan int64, 100),
		updater:  updater,
		log:      log,
	}
}

func (w *activityWorker) Touch(chatID int64) {
	w.mu.Lock()
	defer w.mu.Unlock()

	last, exists := w.lastSeen[chatID]
	now := time.Now()
	if exists && now.Sub(last) < 5*time.Minute {
		return
	}

	select {
	case w.ch <- chatID:
		w.lastSeen[chatID] = now
	default:
		// Очередь заполнена — не обновляем lastSeen, чтобы следующий запрос попробовал снова.
	}
}

func (w *activityWorker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case chatID := <-w.ch:
			updateCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			if err := w.updater.UpdateActivity(updateCtx, chatID); err != nil {
				w.log.Debug("failed updating user activity", "chat_id", chatID, "error", err)
			}
			cancel()
		}
	}
}
