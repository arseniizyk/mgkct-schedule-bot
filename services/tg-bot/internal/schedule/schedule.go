package schedule

import (
	"context"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

type ScheduleUsecase interface {
	GetGroupSchedule(context.Context, int) (*pb.GroupScheduleResponse, error)
}
