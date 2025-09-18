package postgres

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"database/sql"

	sq "github.com/Masterminds/squirrel"
	e "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/errors"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/telegram/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
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
		Values(u.ChatID, u.Username).Suffix("ON CONFLICT (chat_id) DO UPDATE SET username = EXCLUDED.username")

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

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		slog.Error("repo: build to sql", "query", query, "err", err)
		return 0, e.Internal
	}

	var groupId sql.NullInt64
	if err := r.pool.QueryRow(ctx, sqlQuery, args...).Scan(&groupId); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, e.UserNoGroup
		}
		slog.Error("repo: internal from queryRow", "query", sqlQuery, "err", err)
		return 0, e.Internal
	}

	if !groupId.Valid {
		return 0, e.UserNoGroup
	}

	return int(groupId.Int64), nil
}

func (r *UserRepository) SetUserGroup(ctx context.Context, chatID int64, groupID int) error {
	query := r.sb.Insert("users").
		Columns("chat_id", "group_id").
		Values(chatID, groupID).
		Suffix("ON CONFLICT (chat_id) DO UPDATE SET group_id = EXCLUDED.group_id")

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("repo: build to sql: %w", err)
	}

	_, err = r.pool.Exec(ctx, sql, args...)

	return err
}
