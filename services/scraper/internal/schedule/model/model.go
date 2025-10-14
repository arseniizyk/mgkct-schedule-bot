package model

import (
	"errors"
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

var ErrNotFound = errors.New("not found")

type Updated struct {
	Group         *pb.Group
	IsWeekUpdated bool
}

type Weeks struct {
	Prev    time.Time
	Current time.Time
	Next    time.Time
}
