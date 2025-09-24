package bus

import (
	"context"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type Bus struct {
	nc      *nats.Conn
	handler telegram.ScheduleHandlerUsecase
}

func New(handler telegram.ScheduleHandlerUsecase, conn *nats.Conn) *Bus {
	return &Bus{
		nc:      conn,
		handler: handler,
	}
}

func (n *Bus) SubscribeGroupUpdates() {
	n.nc.Subscribe("schedule", func(msg *nats.Msg) {
		group := &pb.GroupScheduleResponse{}
		err := proto.Unmarshal(msg.Data, group)
		if err != nil {
			slog.Error("unmarshalling proto", "err", err)
		}
		if err := n.handler.HandleScheduleUpdate(context.Background(), group); err != nil {
			slog.Error("handle schedule update", "err", err)
		}
	})
}
