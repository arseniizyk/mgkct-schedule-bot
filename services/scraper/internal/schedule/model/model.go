package model

import (
	"errors"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

var ErrNotFound = errors.New("not found")

type Updated struct {
	Group *pb.Group
}
