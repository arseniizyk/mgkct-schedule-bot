package model

import (
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

type Updated struct {
	Group         *pb.Group
	IsWeekUpdated bool
}

type Weeks struct {
	Prev    time.Time
	Current time.Time
	Next    time.Time
}
