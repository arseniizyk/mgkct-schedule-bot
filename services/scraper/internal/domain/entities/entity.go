package entities

import (
	"time"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

type UpdatedGroup struct {
	Group         *pb.Group
	IsWeekUpdated bool
}

type WeekNavigation struct {
	Prev    time.Time
	Current time.Time
	Next    time.Time
}
