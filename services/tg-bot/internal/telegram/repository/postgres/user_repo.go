package postgres

import (
	"context"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type userRepo struct {
	pool *pgxpool.Pool
}

func NewUserRepo(pool *pgxpool.Pool) repository.UserRepository {
	return &userRepo{pool: pool}
}

func (r *userRepo) SaveUser(ctx context.Context, u *models.User) error {
	return nil
}

func (r *userRepo) GetUserGroup(ctx context.Context, chatID int64) (int, error) {
	return 0, nil
}
func (r *userRepo) SetUserGroup(ctx context.Context, chatID int64, groupID int) error {
	return nil
}
