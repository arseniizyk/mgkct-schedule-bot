package postgres

import (
	"context"
	"errors"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) repository.UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) SaveUser(ctx context.Context, u *models.User) error {
	return nil
}

func (r *UserRepository) GetUserGroup(ctx context.Context, chatID int64) (int, error) {
	return 0, nil
}
func (r *UserRepository) SetUserGroup(ctx context.Context, chatID int64, groupID int) error {
	return nil
}
