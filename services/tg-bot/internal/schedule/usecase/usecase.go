package usecase

import (
	"context"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule/transport"
)

type scheduleUseCase struct {
	crawlSvc *transport.GRPCStub
}

func NewScheduleUseCase(stub *transport.GRPCStub) schedule.ScheduleUseCase {
	return &scheduleUseCase{stub}
}

func (sch *scheduleUseCase) GetGroupSchedule(ctx context.Context, num int) (*pb.GroupScheduleResponse, error) {
	resp, err := sch.crawlSvc.GetGroupSchedule(ctx, &pb.GroupScheduleRequest{
		GroupNum: int32(num),
	})

	if err != nil {
		slog.Error("Error from scraper service", "num", num, "err", err)
		return nil, err
	}

	return resp, nil
}
