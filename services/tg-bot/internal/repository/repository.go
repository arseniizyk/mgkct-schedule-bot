package repository

import (
	"context"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
)

type User interface {
	SaveUser(ctx context.Context, u entities.User) error
	AllUsers(ctx context.Context) ([]entities.User, error)
	GroupByChatID(ctx context.Context, chatID int64) (int, error)
	UserIDsByGroupID(ctx context.Context, groupID int) ([]int64, error)
	SetUserGroup(ctx context.Context, chatID int64, groupID int) error
}
