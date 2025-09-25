package transport

import (
	"fmt"
	"log/slog"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type Nats struct {
	nc *nats.Conn
}

func NewNats(cfg *config.Config) (*Nats, error) {
	nc, err := nats.Connect(cfg.NatsURL, nats.Name("scraper"))
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}
	return &Nats{nc: nc}, nil
}

func (n *Nats) PublishScheduleUpdate(group *pb.Group) error {
	slog.Debug("Publishing schedule update", "group_id", group.Id)
	resp := &pb.GroupScheduleResponse{
		Group: group,
	}
	data, err := proto.Marshal(resp)
	if err != nil {
		slog.Error("PublishScheduleUpdate: marshal proto", "group", group, "err", err)
		return err
	}
	return n.nc.Publish("schedule.updates", data)
}

func (n *Nats) Close() error {
	return n.nc.Drain()
}
