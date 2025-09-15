package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository/postgres"
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
	if err := uc.repo.SetUserGroup(ctx, chatID, groupID); err != nil {
		slog.Error("can't save group", "chat_id", chatID, "groupID", groupID, "err", err)
		return err
	}

	return nil
}

func (uc *UserUsecase) GetUserGroup(ctx context.Context, chatID int64) (string, error) {
	_, err := uc.repo.GetUserGroup(ctx, chatID)
	if err != nil {
		slog.Error("can't get user group", "chat_id", chatID, "err", err)
		return "", err
	}

	return "", nil
}

func (uc *UserUsecase) GetGroupScheduleByID(ctx context.Context, groupID int) (*pb.GroupScheduleResponse, error) {
	return uc.scheduleUC.GetGroupSchedule(ctx, groupID)
}

func (uc *UserUsecase) GetGroupScheduleByChatID(ctx context.Context, chatID int64) (*pb.GroupScheduleResponse, error) {
	groupID, err := uc.repo.GetUserGroup(ctx, chatID)
	if err != nil {
		if errors.Is(err, postgres.ErrUserNotFound) {
			return nil, fmt.Errorf("user's group not found chat_id: %d, %w", chatID, err)
		} else {
			return nil, fmt.Errorf("can't get user's group: chat_id: %d, %w", chatID, err)
		}
	}

	return uc.scheduleUC.GetGroupSchedule(ctx, groupID)
}
