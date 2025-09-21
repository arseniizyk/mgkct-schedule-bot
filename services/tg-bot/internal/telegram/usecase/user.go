package usecase

import (
	"context"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository"
)

type UserUsecase struct {
	repo       repository.UserRepository
	scheduleUC schedule.ScheduleUsecase
}

func New(scheduleUC schedule.ScheduleUsecase, repo repository.UserRepository) telegram.UserUsecase {
	return &UserUsecase{
		repo:       repo,
		scheduleUC: scheduleUC,
	}
}

func (uc *UserUsecase) SaveUser(ctx context.Context, u *models.User) error {
	if err := uc.repo.SaveUser(ctx, u); err != nil {
		return err
	}
	return nil
}

func (uc *UserUsecase) SetUserGroup(ctx context.Context, chatID int64, groupID int) error {
	return uc.repo.SetUserGroup(ctx, chatID, groupID)
}

func (uc *UserUsecase) GetGroupScheduleByID(ctx context.Context, groupID int) (*pb.GroupScheduleResponse, error) {
	return uc.scheduleUC.GetGroupSchedule(ctx, groupID)
}

func (uc *UserUsecase) GetGroupScheduleByChatID(ctx context.Context, chatID int64) (*pb.GroupScheduleResponse, error) {
	groupID, err := uc.repo.GetUserGroup(ctx, chatID)
	if err != nil {
		slog.Error("can't get user group", "chat_id", chatID, "err", err)
		return nil, err
	}
	return uc.GetGroupScheduleByID(ctx, groupID)
}
