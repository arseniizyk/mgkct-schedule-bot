package usecase

import (
	"context"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/delivery"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository"
)

type ScheduleHandlerUsecase struct {
	h        *delivery.Handler
	userRepo repository.UserRepository
}

func NewScheduleHandlerUsecase(h *delivery.Handler) telegram.ScheduleHandler {
	return &ScheduleHandlerUsecase{
		h: h,
	}
}

func (uc *ScheduleHandlerUsecase) HandleScheduleUpdate(ctx context.Context, g *pb.GroupScheduleResponse) error {
	users, err := uc.userRepo.GetGroupUsers(ctx, int(g.GroupNum))
	if err != nil {
		slog.Error("can't get users for group", "groupNum", g.GroupNum, "err", err)
		return err
	}

	for _, u := range users {
		return uc.h.SendUpdate(u, g)
	}

	return nil
}
