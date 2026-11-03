package handlers

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	tele "gopkg.in/telebot.v4"
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
			err:       errors.New("something went wrong"),
			wantSec:   0,
			wantFound: false,
		},
		{
			name:      "tele.FloodError value",
			err:       tele.FloodError{RetryAfter: 28},
			wantSec:   28,
			wantFound: true,
		},
		{
			name:      "tele.FloodError pointer",
			err:       &tele.FloodError{RetryAfter: 15},
			wantSec:   15,
			wantFound: true,
		},
		{
			name:      "wrapped string with retry after",
			err:       errors.New("telegram: retry after 26 (429)"),
			wantSec:   26,
			wantFound: true,
		},
		{
			name:      "wrapped string with FLOOD_WAIT",
			err:       errors.New("FLOOD_WAIT_10"),
			wantSec:   0,
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sec, found := extractFloodRetry(tt.err)
			assert.Equal(t, tt.wantFound, found)
			assert.Equal(t, tt.wantSec, sec)
		})
	}
}
