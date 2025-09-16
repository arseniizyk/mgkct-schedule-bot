package usecase

import (
	"context"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule"
)

type ScheduleUsecase struct {
	scraperSvc pb.ScheduleServiceClient
}

func New(stub pb.ScheduleServiceClient) schedule.ScheduleUsecase {
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
