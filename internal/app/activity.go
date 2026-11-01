package app

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/repository"
)

type activityWorker struct {
	mu       sync.Mutex
	lastSeen map[int64]time.Time
	ch       chan int64
	userRepo *repository.UserRepository
	log      *slog.Logger
}

func newActivityWorker(log *slog.Logger, userRepo *repository.UserRepository) *activityWorker {
	return &activityWorker{
		lastSeen: make(map[int64]time.Time),
		ch:       make(chan int64, 100),
		userRepo: userRepo,
		log:      log,
	}
}

func (w *activityWorker) Touch(chatID int64) {
	w.mu.Lock()
	last, exists := w.lastSeen[chatID]
	now := time.Now()
	if exists && now.Sub(last) < 5*time.Minute {
		w.mu.Unlock()
		return
	}
	w.lastSeen[chatID] = now
	w.mu.Unlock()

	select {
	case w.ch <- chatID:
	default:
	}
}

func (w *activityWorker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case chatID := <-w.ch:
			updateCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			if err := w.userRepo.UpdateActivity(updateCtx, chatID); err != nil {
				w.log.Debug("failed updating user activity", "chat_id", chatID, "error", err)
			}
			cancel()
		}
	}
}
