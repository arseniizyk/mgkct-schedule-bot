package middleware

import (
	"context"
	"log/slog"
	"sync"
	"time"

	tele "gopkg.in/telebot.v4"
)

type ActivityUpdater interface {
	UpdateActivity(ctx context.Context, chatID int64) error
}

type ActivityWorker struct {
	mu       sync.Mutex
	lastSeen map[int64]time.Time
	ch       chan int64
	updater  ActivityUpdater
	log      *slog.Logger
}

func NewActivityWorker(log *slog.Logger, updater ActivityUpdater) *ActivityWorker {
	return &ActivityWorker{
		lastSeen: make(map[int64]time.Time),
		ch:       make(chan int64, 100),
		updater:  updater,
		log:      log,
	}
}

func (w *ActivityWorker) Touch(chatID int64) {
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
		// Queue full — do not update lastSeen so next request retries.
	}
}

func (w *ActivityWorker) cleanupOldEntries(cutoff time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for id, last := range w.lastSeen {
		if last.Before(cutoff) {
			delete(w.lastSeen, id)
		}
	}
}

func (w *ActivityWorker) Run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.cleanupOldEntries(time.Now().Add(-1 * time.Hour))
		case chatID := <-w.ch:
			updateCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			if err := w.updater.UpdateActivity(updateCtx, chatID); err != nil {
				w.log.Debug("failed updating user activity", "chat_id", chatID, "error", err)
			}
			cancel()
		}
	}
}

// ActivityMiddleware returns a Telebot middleware that touches user activity for every incoming update.
func ActivityMiddleware(worker *ActivityWorker) tele.MiddlewareFunc {
	return func(next tele.HandlerFunc) tele.HandlerFunc {
		return func(c tele.Context) error {
			if c.Sender() != nil {
				worker.Touch(c.Sender().ID)
			}
			return next(c)
		}
	}
}
