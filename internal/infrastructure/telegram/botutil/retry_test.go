package botutil_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/botutil"
)

func TestExtractFloodRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		wantSec   int
		wantFound bool
	}{
		{
			name:      "nil error",
			err:       nil,
			wantSec:   0,
			wantFound: false,
		},
		{
			name:      "generic error",
			err:       errors.New("generic network error"),
			wantSec:   0,
			wantFound: false,
		},
		{
			name:      "tele.FloodError value",
			err:       tele.FloodError{RetryAfter: 42},
			wantSec:   42,
			wantFound: true,
		},
		{
			name:      "tele.FloodError pointer",
			err:       &tele.FloodError{RetryAfter: 12},
			wantSec:   12,
			wantFound: true,
		},
		{
			name:      "string retry after with seconds",
			err:       errors.New("telegram: retry after 25 (429)"),
			wantSec:   25,
			wantFound: true,
		},
		{
			name:      "string FLOOD_WAIT",
			err:       errors.New("FLOOD_WAIT_15"),
			wantSec:   0,
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sec, found := botutil.ExtractFloodRetry(tt.err)
			assert.Equal(t, tt.wantFound, found)
			assert.Equal(t, tt.wantSec, sec)
		})
	}
}

func TestSendWithFloodRetry(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.DiscardHandler)

	t.Run("success on first try", func(t *testing.T) {
		t.Parallel()
		expectedMsg := &tele.Message{ID: 123}
		msg, err := botutil.SendWithFloodRetry(t.Context(), logger, func() (*tele.Message, error) {
			return expectedMsg, nil
		})
		require.NoError(t, err)
		assert.Equal(t, expectedMsg, msg)
	})

	t.Run("non-flood error returns immediately", func(t *testing.T) {
		t.Parallel()
		customErr := errors.New("chat not found")
		calls := 0
		msg, err := botutil.SendWithFloodRetry(t.Context(), logger, func() (*tele.Message, error) {
			calls++
			return nil, customErr
		})
		require.ErrorIs(t, err, customErr)
		assert.Nil(t, msg)
		assert.Equal(t, 1, calls)
	})

	t.Run("flood retry succeeds on retry", func(t *testing.T) {
		t.Parallel()
		calls := 0
		expectedMsg := &tele.Message{ID: 456}
		msg, err := botutil.SendWithFloodRetry(t.Context(), logger, func() (*tele.Message, error) {
			calls++
			if calls == 1 {
				// RetryAfter 0 will trigger minimal duration in retry loop
				return nil, tele.FloodError{RetryAfter: 0}
			}
			return expectedMsg, nil
		})
		require.NoError(t, err)
		assert.Equal(t, expectedMsg, msg)
		assert.Equal(t, 2, calls)
	})

	t.Run("context cancelled during flood wait", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		go func() {
			time.Sleep(50 * time.Millisecond)
			cancel()
		}()
		_, err := botutil.SendWithFloodRetry(ctx, logger, func() (*tele.Message, error) {
			calls++
			return nil, tele.FloodError{RetryAfter: 5}
		})
		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
