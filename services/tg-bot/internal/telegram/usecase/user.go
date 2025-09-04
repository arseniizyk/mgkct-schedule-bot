package usecase

import (
	"context"
	"log/slog"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository"
)

type UserUseCase interface {
	Save(ctx context.Context, u *models.User) error
	SetGroup(ctx context.Context, chatID int64, groupID int) error
	GetGroup(ctx context.Context, chatID int64) (string, error) // TODO: model for group
}

type userUC struct {
	repo repository.UserRepository
}

func NewUserUseCase(repo repository.UserRepository) UserUseCase {
	return &userUC{repo: repo}
}

func (uc *userUC) Save(ctx context.Context, u *models.User) error {
	if err := uc.repo.SaveUser(ctx, u); err != nil {
		return err
	}

	return nil
}

func (uc *userUC) SetGroup(ctx context.Context, chatID int64, groupID int) error {
	if err := uc.repo.SetUserGroup(ctx, chatID, groupID); err != nil {
		slog.Error("can't save group", "chat_id", chatID, "groupID", groupID, "err", err)
		return err
	}

	return nil
}

func (uc *userUC) GetGroup(ctx context.Context, chatID int64) (string, error) {
	_, err := uc.repo.GetUserGroup(ctx, chatID)
	if err != nil {
		slog.Error("can't get user group", "chat_id", chatID, "err", err)
		return "", err
	}

	return "", nil
}
