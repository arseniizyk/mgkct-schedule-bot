package handlers

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tele "gopkg.in/telebot.v4"
)

type recordingContext struct {
	tele.Context

	sentCalls []sentCall
}

type sentCall struct {
	what any
	opts []any
}

func (c *recordingContext) Send(what any, opts ...any) error {
	c.sentCalls = append(c.sentCalls, sentCall{what: what, opts: opts})
	return nil
}

func TestSendLongMessage_PreservesParseMode(t *testing.T) {
	t.Parallel()

	// Создаем текст длиннее 4096 символов, чтобы получилось минимум 2 чанка
	longHTML := "<b>" + strings.Repeat("a", 5000) + "</b>"
	c := &recordingContext{}

	err := sendLongMessage(c, longHTML, tele.ModeHTML)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(c.sentCalls), 2)

	// Проверяем, что для всех чанков, включая промежуточные, сохранился tele.ModeHTML
	for i, call := range c.sentCalls {
		var hasHTML bool
		for _, opt := range call.opts {
			if s, ok := opt.(string); ok && s == tele.ModeHTML {
				hasHTML = true
				break
			}
		}
		assert.True(t, hasHTML, "chunk %d must preserve tele.ModeHTML", i)
	}
}

func TestExtractParseMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts []any
		want []any
	}{
		{
			name: "html mode",
			opts: []any{tele.ModeHTML},
			want: []any{tele.ModeHTML},
		},
		{
			name: "markdown mode with markup",
			opts: []any{tele.ModeMarkdown, &tele.ReplyMarkup{}},
			want: []any{tele.ModeMarkdown},
		},
		{
			name: "no parse mode",
			opts: []any{&tele.ReplyMarkup{}},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := extractParseMode(tt.opts)
			assert.Equal(t, tt.want, got)
		})
	}
}
