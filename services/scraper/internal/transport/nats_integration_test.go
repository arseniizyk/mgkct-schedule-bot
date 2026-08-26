//go:build integration

package transport

import (
	"context"
	"log/slog"
	"testing"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	tcnats "github.com/testcontainers/testcontainers-go/modules/nats"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func newNATSConn(t *testing.T) (*nats.Conn, func()) {
	t.Helper()

	ctx := context.Background()

	natsContainer, err := tcnats.Run(ctx, "nats:2.11-alpine")
	require.NoError(t, err)

	uri, err := natsContainer.ConnectionString(ctx)
	require.NoError(t, err)

	nc, err := nats.Connect(uri)
	require.NoError(t, err)

	return nc, func() {
		nc.Close()
		_ = natsContainer.Terminate(ctx)
	}
}

func TestPublishScheduleUpdateIntegration(t *testing.T) {
	nc, cleanup := newNATSConn(t)
	defer cleanup()

	tr := NewScheduleTransport(slog.Default(), nil, nc)

	sub, err := nc.SubscribeSync("schedule.updates")
	require.NoError(t, err)

	want := &pb.Group{
		Id:   99,
		Week: timestamppb.New(time.Date(2026, time.August, 24, 0, 0, 0, 0, time.UTC)),
	}

	require.NoError(t, tr.PublishScheduleUpdate(want))

	msg, err := sub.NextMsg(5 * time.Second)
	require.NoError(t, err)

	var got pb.GroupScheduleResponse
	require.NoError(t, proto.Unmarshal(msg.Data, &got))
	require.Equal(t, want.Id, got.Group.Id)
	require.Equal(t, want.Week.AsTime(), got.Group.Week.AsTime())
}

func TestPublishWeekUpdatesIntegration(t *testing.T) {
	nc, cleanup := newNATSConn(t)
	defer cleanup()

	tr := NewWeekTransport(slog.Default(), nil, nc)

	sub, err := nc.SubscribeSync("schedule.week.updates")
	require.NoError(t, err)

	date := time.Date(2026, time.August, 31, 15, 30, 0, 0, time.FixedZone("+0300", 3*3600))
	want := date.UTC().Truncate(24 * time.Hour).Format(time.RFC3339)

	require.NoError(t, tr.PublishWeekUpdates(date))

	msg, err := sub.NextMsg(5 * time.Second)
	require.NoError(t, err)
	require.Equal(t, want, string(msg.Data))
}
