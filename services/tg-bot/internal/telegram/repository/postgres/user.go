package postgres

import (
	"context"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrUserNotFound  = errors.New("user not found")
	ErrGroupNotFound = errors.New("user's group not found")
)

type UserRepository struct {
	pool *pgxpool.Pool
	sb   sq.StatementBuilderType
}

func New(pool *pgxpool.Pool) repository.UserRepository {
	return &UserRepository{
		pool: pool,
		sb:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *UserRepository) SaveUser(ctx context.Context, u *models.User) error {
	query := r.sb.Insert("users").
		Columns("chat_id", "username").
		Values(u.ChatID, u.Username)

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("repo: build query to sql: %w", err)
	}

	_, err = r.pool.Exec(ctx, sql, args...)
	return err
}

func (r *UserRepository) GetUserGroup(ctx context.Context, chatID int64) (int, error) {
	query := r.sb.Select("group_id").
		From("users").
		Where(sq.Eq{"chat_id": chatID})

	sql, args, err := query.ToSql()
	if err != nil {
		return 0, fmt.Errorf("repo: build to sql: %w", err)
	}

	var groupId int
	if err := r.pool.QueryRow(ctx, sql, args...).Scan(&groupId); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrGroupNotFound
		}
		return 0, fmt.Errorf("repo: query user group: %w", err)
	}

	return groupId, nil
}
func (r *UserRepository) SetUserGroup(ctx context.Context, chatID int64, groupID int) error {
	query := r.sb.Update("users").Where(sq.Eq{"chat_id": chatID}).Set("group_id", groupID)

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("repo: build to sql: %w", err)
	}

	_, err = r.pool.Exec(ctx, sql, args...)

	return err
}
