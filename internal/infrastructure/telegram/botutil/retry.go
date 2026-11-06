package botutil

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v4"
)

// ExtractFloodRetry extracts retry-after seconds if err represents a Telegram 429 flood error.
func ExtractFloodRetry(err error) (int, bool) {
	if err == nil {
		return 0, false
	}
	if fErr, ok := errors.AsType[tele.FloodError](err); ok {
		return fErr.RetryAfter, true
	}
	var fErrPtr *tele.FloodError
	if errors.As(err, &fErrPtr) && fErrPtr != nil {
		return fErrPtr.RetryAfter, true
	}
	msg := err.Error()
	if strings.Contains(msg, "retry after") || strings.Contains(msg, "FLOOD_WAIT") {
		if _, after, ok := strings.Cut(msg, "retry after "); ok {
			fields := strings.Fields(after)
			if len(fields) > 0 {
				if sec, parseErr := strconv.Atoi(fields[0]); parseErr == nil {
					return sec, true
				}
			}
		}
		return 0, true
	}
	return 0, false
}

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
		if retryAfter, ok := ExtractFloodRetry(err); ok {
			waitDur := time.Duration(retryAfter) * time.Second
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
