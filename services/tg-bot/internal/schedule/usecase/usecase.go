package usecase

import (
	"context"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule/transport"
)

type ScheduleUsecase struct {
	scraperSvc *transport.GRPCStub
}

func New(stub *transport.GRPCStub) schedule.ScheduleUsecase {
	return &ScheduleUsecase{stub}
}

func (sch *ScheduleUsecase) GetGroupSchedule(ctx context.Context, num int) (*pb.GroupScheduleResponse, error) {
	resp, err := sch.scraperSvc.GetGroupSchedule(ctx, &pb.GroupScheduleRequest{
		GroupNum: int32(num),
	})

	if err != nil {
		slog.Error("Error from scraper service", "num", num, "err", err)
		return nil, err
	}

	return resp, nil
}
