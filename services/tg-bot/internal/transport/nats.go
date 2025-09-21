package transport

import (
	"context"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type NatsConsumer struct {
	nc      *nats.Conn
	handler telegram.ScheduleHandler
}

func NewNatsConsumer(handler telegram.ScheduleHandler, conn *nats.Conn) (*NatsConsumer, error) {
	return &NatsConsumer{
		nc:      conn,
		handler: handler,
	}, nil
}

func (n *NatsConsumer) SubscribeGroupUpdates() {
	n.nc.Subscribe("schedule", func(msg *nats.Msg) {
		group := &pb.GroupScheduleResponse{}
		proto.Unmarshal(msg.Data, group)
		if err := n.handler.HandleScheduleUpdate(context.Background(), group); err != nil {
			slog.Error("handle schedule update", "err", err)
		}
	})
}
