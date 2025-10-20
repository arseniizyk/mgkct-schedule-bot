package repository

import (
	"context"
	"errors"

	"database/sql"

	sq "github.com/Masterminds/squirrel"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User interface {
	SelectAll(ctx context.Context) ([]models.User, error)
	Save(ctx context.Context, u models.User) error
	GetGroup(ctx context.Context, chatID int64) (int, error)
	SetGroup(ctx context.Context, chatID int64, groupID int) error
	GetUsersByGroup(ctx context.Context, groupID int) ([]int64, error)
}

type repository struct {
	pool *pgxpool.Pool
	sb   sq.StatementBuilderType
}

func New(pool *pgxpool.Pool) User {
	return &repository{
		pool: pool,
		sb:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *repository) Save(ctx context.Context, u models.User) error {
	query := r.sb.Insert("users").
		Columns("chat_id", "username").
		Values(u.ChatID, u.Username).Suffix("ON CONFLICT (chat_id) DO UPDATE SET username = EXCLUDED.username")

	sql, args, err := query.ToSql()
	if err != nil {
		return wrap(ErrBuildQuery, err)
	}

	_, err = r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return wrap(ErrExec, err)
	}

	return nil
}

func (r *repository) SelectAll(ctx context.Context) ([]models.User, error) {
	query := r.sb.Select("chat_id, group_id").
		From("users")

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, wrap(ErrBuildQuery, err)
	}

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, wrap(ErrQuery, err)
	}

	var users []models.User
	for rows.Next() {
		var chatID int64
		var groupID *int
		if err := rows.Scan(&chatID, &groupID); err != nil {
			return nil, wrap(ErrScan, err, "chat_id")
		}

		group := 0
		if groupID != nil {
			group = *groupID
		}

		users = append(users, models.User{
			ChatID: chatID,
			Group:  group,
		})
	}

	return users, nil
}

func (r *repository) GetGroup(ctx context.Context, chatID int64) (int, error) {
	query := r.sb.Select("group_id").
		From("users").
		Where(sq.Eq{"chat_id": chatID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return 0, wrap(ErrBuildQuery, err)
	}

	var groupId sql.NullInt64
	if err := r.pool.QueryRow(ctx, sqlQuery, args...).Scan(&groupId); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, wrap(ErrNoGroup, err)
		}

		return 0, wrap(ErrScan, err, "group_id")
	}

	if !groupId.Valid {
		return 0, wrap(ErrNoGroup, err)
	}

	return int(groupId.Int64), nil
}

func (r *repository) GetUsersByGroup(ctx context.Context, groupID int) ([]int64, error) {
	query := r.sb.Select("chat_id").
		From("users").
		Where(sq.Eq{"group_id": groupID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, wrap(ErrBuildQuery, err)
	}

	rows, err := r.pool.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, wrap(ErrQuery, err)
	}
	defer rows.Close()

	var users []int64
	for rows.Next() {
		var u int64
		if err := rows.Scan(&u); err != nil {
			return nil, wrap(ErrScan, err, "chat_id")
		}
		users = append(users, u)
	}

	if err := rows.Err(); err != nil {
		return nil, wrap(ErrRows, err)
	}

	return users, nil
}

func (r *repository) SetGroup(ctx context.Context, chatID int64, groupID int) error {
	query := r.sb.Insert("users").
		Columns("chat_id", "group_id").
		Values(chatID, groupID).
		Suffix("ON CONFLICT (chat_id) DO UPDATE SET group_id = EXCLUDED.group_id")

	sql, args, err := query.ToSql()
	if err != nil {
		return wrap(ErrBuildQuery, err)
	}

	_, err = r.pool.Exec(ctx, sql, args...)
	if err != nil {
		return wrap(ErrExec, err)
	}

	return nil
}
