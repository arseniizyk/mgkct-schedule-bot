package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	scraperpb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/schedule"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository/postgres"
)

type UserUseCase interface {
	SaveUser(ctx context.Context, u *models.User) error
	SetUserGroup(ctx context.Context, chatID int64, groupID int) error
	GetUserGroup(ctx context.Context, chatID int64) (string, error) // TODO: model for group
	GetGroupScheduleByID(ctx context.Context, groupID int) (*scraperpb.GroupScheduleResponse, error)
	GetGroupScheduleByChatID(ctx context.Context, chatID int64) (*scraperpb.GroupScheduleResponse, error)
}

type userUC struct {
	repo       repository.UserRepository
	scheduleUC schedule.ScheduleUseCase
}

func NewUserUseCase(scheduleUC schedule.ScheduleUseCase, repo repository.UserRepository) UserUseCase {
	return &userUC{
		repo:       repo,
		scheduleUC: scheduleUC,
	}
}

func (uc *userUC) SaveUser(ctx context.Context, u *models.User) error {
	if err := uc.repo.SaveUser(ctx, u); err != nil {
		return err
	}

	return nil
}

func (uc *userUC) SetUserGroup(ctx context.Context, chatID int64, groupID int) error {
	if err := uc.repo.SetUserGroup(ctx, chatID, groupID); err != nil {
		slog.Error("can't save group", "chat_id", chatID, "groupID", groupID, "err", err)
		return err
	}

	return nil
}

func (uc *userUC) GetUserGroup(ctx context.Context, chatID int64) (string, error) {
	_, err := uc.repo.GetUserGroup(ctx, chatID)
	if err != nil {
		slog.Error("can't get user group", "chat_id", chatID, "err", err)
		return "", err
	}

	return "", nil
}

func (uc *userUC) GetGroupScheduleByID(ctx context.Context, groupID int) (*scraperpb.GroupScheduleResponse, error) {
	return uc.scheduleUC.GetGroupSchedule(ctx, groupID)
}

func (uc *userUC) GetGroupScheduleByChatID(ctx context.Context, chatID int64) (*scraperpb.GroupScheduleResponse, error) {
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
