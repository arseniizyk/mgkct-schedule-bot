package repository

import (
	"context"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
)

type UserRepository interface {
	SaveUser(ctx context.Context, u *models.User) error
	GetUserGroup(ctx context.Context, chatID int64) (int, error)
	SetUserGroup(ctx context.Context, chatID int64, groupID int) error
}
