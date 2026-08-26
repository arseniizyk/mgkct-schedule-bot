package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	sq "github.com/Masterminds/squirrel"
)

type UserRepository struct {
	pool *pgxpool.Pool
	sb   sq.StatementBuilderType
}

func New(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{
		pool: pool,
		sb:   sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
	}
}

func (r *UserRepository) SaveUser(ctx context.Context, u entities.User) error {
	const op = "repository.user.UserRepository.Save"

	query := r.sb.Insert("users").
		Columns("chat_id", "username").
		Values(u.ChatID, u.Username).
		Suffix("ON CONFLICT (chat_id) DO UPDATE SET username = EXCLUDED.username")

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	if _, err = r.pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	return nil
}

func (r *UserRepository) AllUsers(ctx context.Context) ([]entities.User, error) {
	const op = "repository.user.UserRepository.SelectAllUsers"

	query := r.sb.Select("chat_id, group_id").From("users")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	rows, err := r.pool.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}
	defer rows.Close()

	var users []entities.User

	for rows.Next() {
		var (
			chatID  int64
			groupID sql.NullInt64
		)

		if err := rows.Scan(&chatID, &groupID); err != nil {
			return nil, fmt.Errorf("%s: failed on scanning chat_id, group_id: %w", op, err)
		}

		group := 0

		if groupID.Valid {
			group = int(groupID.Int64)
		}

		users = append(users, entities.User{
			ChatID: chatID,
			Group:  group,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: error iterating rows: %w", op, err)
	}

	return users, nil
}

func (r *UserRepository) GroupByChatID(ctx context.Context, chatID int64) (int, error) {
	const op = "repository.user.UserRepository.GetGroup"

	query := r.sb.Select("group_id").From("users").Where(sq.Eq{"chat_id": chatID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return 0, fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	var groupID sql.NullInt64
	if err := r.pool.QueryRow(ctx, sqlQuery, args...).Scan(&groupID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, domainerr.ErrUserNoGroup
		}

		return 0, fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	if !groupID.Valid {
		return 0, domainerr.ErrUserNoGroup
	}

	return int(groupID.Int64), nil
}

func (r *UserRepository) UserIDsByGroupID(ctx context.Context, groupID int) ([]int64, error) {
	const op = "repository.user.UserRepository.GetUsersByGroupID"

	query := r.sb.Select("chat_id").From("users").Where(sq.Eq{"group_id": groupID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	rows, err := r.pool.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}
	defer rows.Close()

	usersID := make([]int64, 0, 5)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("%s: failed on scanning chat_id: %w", op, err)
		}
		usersID = append(usersID, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: error scanning rows: %w", op, err)
	}

	return usersID, nil
}

func (r *UserRepository) SetUserGroup(ctx context.Context, chatID int64, groupID int) error {
	const op = "repository.user.UserRepository.SetUserGroup"

	query := r.sb.Insert("users").
		Columns("chat_id", "group_id").
		Values(chatID, groupID).
		Suffix("ON CONFLICT (chat_id) DO UPDATE SET group_id = EXCLUDED.group_id")

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	if _, err = r.pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	return nil
}
