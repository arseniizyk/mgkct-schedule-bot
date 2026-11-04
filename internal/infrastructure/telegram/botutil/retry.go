package botutil

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	tele "gopkg.in/telebot.v4"
)

// SendWithFloodRetry executes sendFunc and retries on Telegram 429 FloodError up to 3 times.
func SendWithFloodRetry(ctx context.Context, log *slog.Logger, sendFunc func() (*tele.Message, error)) (*tele.Message, error) {
	const maxRetries = 3
	var lastErr error
	for range maxRetries {
		msg, err := sendFunc()
		if err == nil {
			return msg, nil
		}
		lastErr = err
		if floodErr, ok := errors.AsType[tele.FloodError](err); ok {
			waitDur := time.Duration(floodErr.RetryAfter) * time.Second
			if waitDur <= 0 {
				waitDur = time.Second
			}
			log.WarnContext(ctx, "telegram flood wait 429 received, sleeping", "retry_after", waitDur)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitDur):
				continue
			}
		}
		return nil, err
	}
	if lastErr != nil {
		return nil, fmt.Errorf("max retries exceeded for flood wait: %w", lastErr)
	}
	return nil, errors.New("max retries exceeded for flood wait")
}
