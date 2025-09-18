package usecase

import (
	"context"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	e "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ScheduleUsecase struct {
	scraperSvc pb.ScheduleServiceClient
}

func New(stub pb.ScheduleServiceClient) schedule.ScheduleUsecase {
	return &ScheduleUsecase{stub}
}

func (sch *ScheduleUsecase) GetGroupSchedule(ctx context.Context, groupID int) (*pb.GroupScheduleResponse, error) {
	resp, err := sch.scraperSvc.GetGroupSchedule(ctx, &pb.GroupScheduleRequest{
		GroupNum: int32(groupID),
	})

	if err != nil {
		st, ok := status.FromError(err)
		if !ok {
			slog.Error("undefined error from scraper", "group_id", groupID, "err", err)
			return nil, e.ErrScraperInternal
		}

		switch st.Code() {
		case codes.NotFound:
			slog.Warn("group not found from scraper", "err", st.Message(), "code", st.Code(), "group_id", groupID)
			return nil, e.ErrGroupNotFound
		case codes.Unavailable:
			slog.Error("scraper unavailable", "err", st.Message(), "code", st.Code(), "group_id", groupID)
			return nil, e.ErrScraperInternal
		default:
			slog.Error("undefined error from scraper", "err", st.Message(), "code", st.Code(), "group_id", groupID)
			return nil, e.ErrScraperInternal
		}
	}

	return resp, nil
}
