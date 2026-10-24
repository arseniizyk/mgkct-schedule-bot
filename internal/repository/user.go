package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	sq "github.com/Masterminds/squirrel"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
)

type UserRepository struct {
	db *sql.DB
	sb sq.StatementBuilderType
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{
		db: db,
		sb: sq.StatementBuilder.PlaceholderFormat(sq.Question),
	}
}

func (r *UserRepository) SaveUser(ctx context.Context, u entities.User) error {
	const op = "repository.UserRepository.SaveUser"

	query := r.sb.Insert("users").
		Columns("chat_id", "username").
		Values(u.ChatID, u.Username).
		Suffix("ON CONFLICT(chat_id) DO UPDATE SET username = excluded.username")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	if _, err = r.db.ExecContext(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	return nil
}

func (r *UserRepository) AllUsers(ctx context.Context) ([]entities.User, error) {
	const op = "repository.UserRepository.AllUsers"

	query := r.sb.Select("chat_id, group_id, subgroup, teacher_name").From("users")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}
	defer rows.Close()

	var users []entities.User

	for rows.Next() {
		var (
			chatID      int64
			groupID     sql.NullInt64
			subgroup    sql.NullInt64
			teacherName sql.NullString
		)

		if err := rows.Scan(&chatID, &groupID, &subgroup, &teacherName); err != nil {
			return nil, fmt.Errorf("%s: failed scanning chat_id, group_id, subgroup, teacher_name: %w", op, err)
		}

		group := 0
		if groupID.Valid {
			group = int(groupID.Int64)
		}

		sg := 0
		if subgroup.Valid {
			sg = int(subgroup.Int64)
		}

		tn := ""
		if teacherName.Valid {
			tn = teacherName.String
		}

		users = append(users, entities.User{
			ChatID:      chatID,
			Group:       group,
			Subgroup:    sg,
			TeacherName: tn,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: error iterating rows: %w", op, err)
	}

	return users, nil
}

func (r *UserRepository) GroupByChatID(ctx context.Context, chatID int64) (int, error) {
	const op = "repository.UserRepository.GroupByChatID"

	query := r.sb.Select("group_id").From("users").Where(sq.Eq{"chat_id": chatID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return 0, fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	var groupID sql.NullInt64
	if err := r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(&groupID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, domainerr.ErrUserNoGroup
		}
		return 0, fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	if !groupID.Valid {
		return 0, domainerr.ErrUserNoGroup
	}

	return int(groupID.Int64), nil
}

func (r *UserRepository) SubgroupByChatID(ctx context.Context, chatID int64) (int, error) {
	const op = "repository.UserRepository.SubgroupByChatID"

	query := r.sb.Select("subgroup").From("users").Where(sq.Eq{"chat_id": chatID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return 0, fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	var subgroup sql.NullInt64
	if err := r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(&subgroup); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	if !subgroup.Valid {
		return 0, nil
	}

	return int(subgroup.Int64), nil
}

func (r *UserRepository) GroupAndSubgroupByChatID(ctx context.Context, chatID int64) (int, int, error) {
	const op = "repository.UserRepository.GroupAndSubgroupByChatID"

	query := r.sb.Select("group_id, subgroup").From("users").Where(sq.Eq{"chat_id": chatID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return 0, 0, fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	var (
		groupID  sql.NullInt64
		subgroup sql.NullInt64
	)
	if err := r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(&groupID, &subgroup); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, 0, domainerr.ErrUserNoGroup
		}
		return 0, 0, fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	if !groupID.Valid {
		return 0, 0, domainerr.ErrUserNoGroup
	}

	sg := 0
	if subgroup.Valid {
		sg = int(subgroup.Int64)
	}

	return int(groupID.Int64), sg, nil
}

func (r *UserRepository) UserIDsByGroupID(ctx context.Context, groupID int) ([]int64, error) {
	const op = "repository.UserRepository.UserIDsByGroupID"

	query := r.sb.Select("chat_id").From("users").Where(sq.Eq{"group_id": groupID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}
	defer rows.Close()

	usersID := make([]int64, 0, 10)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("%s: failed scanning chat_id: %w", op, err)
		}
		usersID = append(usersID, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: error scanning rows: %w", op, err)
	}

	return usersID, nil
}

func (r *UserRepository) SetUserGroup(ctx context.Context, chatID int64, groupID int, subgroup int) error {
	const op = "repository.UserRepository.SetUserGroup"

	query := r.sb.Insert("users").
		Columns("chat_id", "group_id", "subgroup", "teacher_name").
		Values(chatID, groupID, subgroup, nil).
		Suffix("ON CONFLICT(chat_id) DO UPDATE SET group_id = excluded.group_id, subgroup = excluded.subgroup, teacher_name = NULL")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	if _, err = r.db.ExecContext(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	return nil
}

func (r *UserRepository) SetUserSubgroup(ctx context.Context, chatID int64, subgroup int) error {
	const op = "repository.UserRepository.SetUserSubgroup"

	query := r.sb.Insert("users").
		Columns("chat_id", "subgroup").
		Values(chatID, subgroup).
		Suffix("ON CONFLICT(chat_id) DO UPDATE SET subgroup = excluded.subgroup")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	if _, err = r.db.ExecContext(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	return nil
}

func (r *UserRepository) SetTeacher(ctx context.Context, chatID int64, teacherName string) error {
	const op = "repository.UserRepository.SetTeacher"

	query := r.sb.Insert("users").
		Columns("chat_id", "teacher_name", "group_id", "subgroup").
		Values(chatID, teacherName, nil, 0).
		Suffix("ON CONFLICT(chat_id) DO UPDATE SET teacher_name = excluded.teacher_name, group_id = NULL, subgroup = 0")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	if _, err = r.db.ExecContext(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	return nil
}

func (r *UserRepository) GetTeacher(ctx context.Context, chatID int64) (string, error) {
	const op = "repository.UserRepository.GetTeacher"

	query := r.sb.Select("teacher_name").From("users").Where(sq.Eq{"chat_id": chatID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return "", fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	var teacherName sql.NullString
	if err := r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(&teacherName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	const op = "repository.UserRepository.GetState"

	query := r.sb.Select("state").From("users").Where(sq.Eq{"chat_id": chatID})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return "", fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	var state sql.NullString
	if err := r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(&state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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
	const op = "repository.UserRepository.SetState"

	query := r.sb.Insert("users").
		Columns("chat_id", "state").
		Values(chatID, state).
		Suffix("ON CONFLICT(chat_id) DO UPDATE SET state = excluded.state")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	if _, err = r.db.ExecContext(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}

	return nil
}

func (r *UserRepository) UserIDsByTeacherName(ctx context.Context, teacherName string) ([]int64, error) {
	const op = "repository.UserRepository.UserIDsByTeacherName"

	query := r.sb.Select("chat_id").From("users").Where(sq.Eq{"teacher_name": teacherName})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to build sql query: %w", op, err)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: failed to execute sql query: %w", op, err)
	}
	defer rows.Close()

	userIDs := make([]int64, 0, 10)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("%s: failed scanning chat_id: %w", op, err)
		}
		userIDs = append(userIDs, id)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: error scanning rows: %w", op, err)
	}

	return userIDs, nil
}

func (r *UserRepository) SetNotifyEvening(ctx context.Context, chatID int64, enabled bool) error {
	const op = "repository.UserRepository.SetNotifyEvening"

	val := 0
	if enabled {
		val = 1
	}

	query := r.sb.Insert("users").
		Columns("chat_id", "notify_evening").
		Values(chatID, val).
		Suffix("ON CONFLICT(chat_id) DO UPDATE SET notify_evening = excluded.notify_evening")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: build query: %w", op, err)
	}

	if _, err = r.db.ExecContext(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("%s: exec query: %w", op, err)
	}

	return nil
}

func (r *UserRepository) GetNotifyEvening(ctx context.Context, chatID int64) (bool, error) {
	const op = "repository.UserRepository.GetNotifyEvening"

	query := r.sb.Select("notify_evening").From("users").Where(sq.Eq{"chat_id": chatID})
	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return true, fmt.Errorf("%s: build query: %w", op, err)
	}

	var notify sql.NullInt64
	if err := r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(&notify); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true, nil
		}
		return true, fmt.Errorf("%s: query row: %w", op, err)
	}

	if !notify.Valid {
		return true, nil
	}
	return notify.Int64 == 1, nil
}

func (r *UserRepository) UsersForEveningNotification(ctx context.Context) ([]entities.User, error) {
	const op = "repository.UserRepository.UsersForEveningNotification"

	query := r.sb.Select("chat_id, group_id, subgroup, teacher_name").
		From("users").
		Where(sq.Eq{"notify_evening": 1}).
		Where(sq.Or{
			sq.Gt{"group_id": 0},
			sq.NotEq{"teacher_name": ""},
		})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build query: %w", op, err)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: query users: %w", op, err)
	}
	defer rows.Close()

	var users []entities.User
	for rows.Next() {
		var (
			chatID      int64
			groupID     sql.NullInt64
			subgroup    sql.NullInt64
			teacherName sql.NullString
		)
		if err := rows.Scan(&chatID, &groupID, &subgroup, &teacherName); err != nil {
			return nil, fmt.Errorf("%s: scan row: %w", op, err)
		}
		var g int
		if groupID.Valid {
			g = int(groupID.Int64)
		}
		var sg int
		if subgroup.Valid {
			sg = int(subgroup.Int64)
		}
		var tn string
		if teacherName.Valid {
			tn = teacherName.String
		}
		users = append(users, entities.User{
			ChatID:        chatID,
			Group:         g,
			Subgroup:      sg,
			TeacherName:   tn,
			NotifyEvening: true,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows err: %w", op, err)
	}
	return users, nil
}
