package user

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/errors"
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

	query := r.sb.Select("chat_id, group_id, teacher_name").From("users")

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
			chatID      int64
			groupID     sql.NullInt64
			teacherName sql.NullString
		)

		if err := rows.Scan(&chatID, &groupID, &teacherName); err != nil {
			return nil, fmt.Errorf("%s: failed on scanning chat_id, group_id, teacher_name: %w", op, err)
		}

		group := 0
		if groupID.Valid {
			group = int(groupID.Int64)
		}

		tn := ""
		if teacherName.Valid {
			tn = teacherName.String
		}

		users = append(users, entities.User{
			ChatID:      chatID,
			Group:       group,
			TeacherName: tn,
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

func (r *UserRepository) SetTeacher(ctx context.Context, chatID int64, teacherName string) error {
	const op = "repository.user.UserRepository.SetTeacher"

	query := r.sb.Insert("users").
		Columns("chat_id", "teacher_name").
		Values(chatID, teacherName).
		Suffix("ON CONFLICT (chat_id) DO UPDATE SET teacher_name = EXCLUDED.teacher_name")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	if _, err = r.pool.Exec(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	return nil
}

func (r *UserRepository) GetTeacher(ctx context.Context, chatID int64) (string, error) {
	const op = "repository.user.UserRepository.GetTeacher"

	query := r.sb.Select("teacher_name").From("users").Where(sq.Eq{"chat_id": chatID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return "", fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	var teacherName sql.NullString
	if err := r.pool.QueryRow(ctx, sqlQuery, args...).Scan(&teacherName); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", domainerr.ErrUserNoTeacher
		}

		return "", fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	if !teacherName.Valid {
		return "", domainerr.ErrUserNoTeacher
	}

	return teacherName.String, nil
}

func (r *UserRepository) GetState(ctx context.Context, chatID int64) (string, error) {
	const op = "repository.user.UserRepository.GetState"

	query := r.sb.Select("state").From("users").Where(sq.Eq{"chat_id": chatID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return "", fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	var state sql.NullString
	if err := r.pool.QueryRow(ctx, sqlQuery, args...).Scan(&state); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}

		return "", fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	if !state.Valid {
		return "", nil
	}

	return state.String, nil
}

func (r *UserRepository) SetState(ctx context.Context, chatID int64, state string) error {
	const op = "repository.user.UserRepository.SetState"

	query := r.sb.Insert("users").
		Columns("chat_id", "state").
		Values(chatID, state).
		Suffix("ON CONFLICT (chat_id) DO UPDATE SET state = EXCLUDED.state")

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	if _, err = r.pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	return nil
}

func (r *UserRepository) UserIDsByTeacherName(ctx context.Context, teacherName string) ([]int64, error) {
	const op = "repository.user.UserRepository.UserIDsByTeacherName"

	query := r.sb.Select("chat_id").From("users").Where(sq.Eq{"teacher_name": teacherName})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	rows, err := r.pool.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}
	defer rows.Close()

	userIDs := make([]int64, 0, 5)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("%s: failed on scanning chat_id: %w", op, err)
		}
		userIDs = append(userIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: error scanning rows: %w", op, err)
	}

	return userIDs, nil
}
