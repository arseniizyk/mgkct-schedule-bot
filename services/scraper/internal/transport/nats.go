package server

import (
	"fmt"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

type Nats struct {
	nc *nats.Conn
}

func NewNats() (*Nats, error) {
	nc, err := nats.Connect("nats://nats:4222")
	if err != nil {
		return nil, fmt.Errorf("connect to NATS: %w", err)
	}
	return &Nats{nc: nc}, nil
}

func (n *Nats) PublishScheduleUpdate(group *models.Group) error {
	resp := &pb.GroupScheduleResponse{
		GroupNum: int32(group.GroupNum),
		Week:     group.Week,
		Day:      daysToProto(group.Days),
	}
	data, err := proto.Marshal(resp)
	if err != nil {
		slog.Error("PublishScheduleUpdate: marshal proto", "group", group, "err", err)
		return err
	}
	return n.nc.Publish("schedule", data)
}

func (n *Nats) Close() {
	n.nc.Drain()
}
