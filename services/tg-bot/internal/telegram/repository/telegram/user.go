package telegram

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"database/sql"

	sq "github.com/Masterminds/squirrel"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repository struct {
	pool *pgxpool.Pool
	sb   sq.StatementBuilderType
}

func New(pool *pgxpool.Pool) *repository {
	return &repository{
		pool: pool,
		sb:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *repository) SaveUser(ctx context.Context, u *models.User) error {
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

func (r *repository) GetUserGroup(ctx context.Context, chatID int64) (int, error) {
	query := r.sb.Select("group_id").
		From("users").
		Where(sq.Eq{"chat_id": chatID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		slog.Error("repo: build to sql", "query", query, "err", err)
		return 0, models.ErrInternal
	}

	var groupId sql.NullInt64
	if err := r.pool.QueryRow(ctx, sqlQuery, args...).Scan(&groupId); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, models.ErrUserNoGroup
		}
		slog.Error("repo: internal from queryRow", "query", sqlQuery, "err", err)
		return 0, models.ErrInternal
	}

	if !groupId.Valid {
		return 0, models.ErrUserNoGroup
	}

	return int(groupId.Int64), nil
}

func (r *repository) GetGroupUsers(ctx context.Context, groupID int) ([]int64, error) {
	query := r.sb.Select("chat_id").
		From("users").
		Where(sq.Eq{"group_id": groupID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		slog.Error("repo: build to sql: ", "query", query, "err", err)
		return nil, models.ErrInternal
	}

	rows, err := r.pool.Query(ctx, sqlQuery, args...)
	if err != nil {
		slog.Error("repo: query db", "err", err)
		return nil, models.ErrInternal
	}
	defer rows.Close()

	var users []int64
	for rows.Next() {
		var u int64
		if err := rows.Scan(&u); err != nil {
			slog.Error("repo: scan row", "err", err)
			return nil, models.ErrInternal
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		slog.Error("repo: rows iteration", "err", err)
		return nil, models.ErrInternal
	}

	return users, nil
}

func (r *repository) SetUserGroup(ctx context.Context, chatID int64, groupID int) error {
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
