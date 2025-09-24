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

func NewScheduleHandlerUsecase(h *delivery.Handler, userRepo repository.UserRepository) telegram.ScheduleHandlerUsecase {
	return &ScheduleHandlerUsecase{
		h:        h,
		userRepo: userRepo,
	}
}

func (uc *ScheduleHandlerUsecase) HandleScheduleUpdate(ctx context.Context, g *pb.GroupScheduleResponse) error {
	users, err := uc.userRepo.GetGroupUsers(ctx, int(g.Group.Id))
	if err != nil {
		slog.Error("can't get users for group", "groupNum", g.Group.Id, "err", err)
		return err
	}

	for _, u := range users {
		err := uc.h.SendUpdate(u, g)
		if err != nil {
			slog.Error("failed to send update to user", "userId", u, "err", err)
		}
	}

	return nil
}
