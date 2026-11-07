package mtproto_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/gotd/td/bin"
	"github.com/gotd/td/tg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/telegram/mtproto"
)

func TestSendMediaWidgetRequest_Encode(t *testing.T) {
	req := &mtproto.SendMediaWidgetRequest{
		Peer:     &tg.InputPeerUser{UserID: 12345, AccessHash: 67890},
		HTML:     "<html><body><h1>Widget Test</h1></body></html>",
		Message:  "Widget caption",
		RandomID: 99999,
	}

	var buf bin.Buffer
	err := req.Encode(&buf)
	require.NoError(t, err)

	bytes := buf.Raw()
	assert.Greater(t, len(bytes), 20)

	// Verify messages.sendMedia#330e77f constructor id
	readBuf := bin.Buffer{Buf: bytes}
	id, err := readBuf.ID()
	require.NoError(t, err)
	assert.Equal(t, uint32(0x330e77f), id)
}

func TestClient_NotReadyError(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	client := mtproto.NewClient(logger, "fake_token", 0, "")

	assert.False(t, client.IsReady())

	err := client.SendWidget(context.Background(), 12345, "<html></html>", "Caption")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not ready")
}
