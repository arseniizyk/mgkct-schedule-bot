package schedule

import (
	"context"

	scraperpb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
)

type ScheduleUseCase interface {
	GetGroupSchedule(context.Context, int) (*scraperpb.GroupScheduleResponse, error)
}
