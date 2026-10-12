package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
)

type TeacherScheduleRepository struct {
	db *sql.DB
	sb squirrel.StatementBuilderType
}

func NewTeacherScheduleRepository(db *sql.DB) *TeacherScheduleRepository {
	return &TeacherScheduleRepository{
		db: db,
		sb: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question),
	}
}

func (r *TeacherScheduleRepository) Save(ctx context.Context, name string, week time.Time, schedule *entities.Teacher) error {
	const op = "repository.TeacherScheduleRepository.Save"

	data, err := json.Marshal(schedule)
	if err != nil {
		return fmt.Errorf("%s: marshal teacher schedule: %w", op, err)
	}

	weekStr := week.Format("2006-01-02")
	query := r.sb.Insert("teacher_schedules").
		Columns("name", "week", "schedule").
		Values(name, weekStr, string(data)).
		Suffix("ON CONFLICT(name, week) DO UPDATE SET schedule = excluded.schedule, updated_at = CURRENT_TIMESTAMP")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	if _, err := r.db.ExecContext(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("%s: exec sql query: %w", op, err)
	}

	return nil
}

func (r *TeacherScheduleRepository) GetByWeek(ctx context.Context, name string, week time.Time) (*entities.Teacher, error) {
	const op = "repository.TeacherScheduleRepository.GetByWeek"

	weekStr := week.Format("2006-01-02")
	query := r.sb.Select("schedule").
		From("teacher_schedules").
		Where(squirrel.Eq{"name": name}).
		Where(squirrel.LtOrEq{"week": weekStr}).
		OrderBy("week DESC").
		Limit(1)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	var raw string
	if err := r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(&raw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainerr.ErrWeekNotFound
		}
		return nil, fmt.Errorf("%s: get teacher schedule: %w", op, err)
	}

	var t entities.Teacher
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return nil, fmt.Errorf("%s: unmarshal teacher schedule: %w", op, err)
	}

	return &t, nil
}

func (r *TeacherScheduleRepository) GetLatest(ctx context.Context, name string) (*entities.Teacher, error) {
	const op = "repository.TeacherScheduleRepository.GetLatest"

	query := r.sb.Select("schedule").
		From("teacher_schedules").
		Where(squirrel.Eq{"name": name}).
		OrderBy("week DESC").
		Limit(1)

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	var raw string
	err = r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domainerr.ErrScheduleNotFound
		}
		return nil, fmt.Errorf("%s: get latest teacher schedule: %w", op, err)
	}

	var t entities.Teacher
	if err := json.Unmarshal([]byte(raw), &t); err != nil {
		return nil, fmt.Errorf("%s: unmarshal teacher schedule: %w", op, err)
	}

	return &t, nil
}

func (r *TeacherScheduleRepository) GetWeeks(ctx context.Context, name string, week time.Time) (entities.WeekNavigation, error) {
	const op = "repository.TeacherScheduleRepository.GetWeeks"

	week = time.Date(week.Year(), week.Month(), week.Day(), 0, 0, 0, 0, time.UTC)

	if week.IsZero() {
		query := r.sb.Select("week").
			From("teacher_schedules").
			Where(squirrel.Eq{"name": name}).
			OrderBy("week DESC").
			Limit(2)

		sqlQuery, args, err := query.ToSql()
		if err != nil {
			return entities.WeekNavigation{}, fmt.Errorf("%s: failed to create query: %w", op, err)
		}

		rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
		if err != nil {
			return entities.WeekNavigation{}, fmt.Errorf("%s: repository sql query: %w", op, err)
		}
		defer rows.Close()

		var weeks []time.Time
		for rows.Next() {
			var wStr string
			if err := rows.Scan(&wStr); err != nil {
				return entities.WeekNavigation{}, fmt.Errorf("%s: repository scan rows: %w", op, err)
			}
			w, err := time.Parse("2006-01-02", wStr)
			if err != nil {
				return entities.WeekNavigation{}, fmt.Errorf("%s: parse date %s: %w", op, wStr, err)
			}
			weeks = append(weeks, w)
		}
		if err := rows.Err(); err != nil {
			return entities.WeekNavigation{}, fmt.Errorf("%s: sql rows err: %w", op, err)
		}

		if len(weeks) < 2 {
			return entities.WeekNavigation{}, domainerr.ErrNoAvailableWeeks
		}

		return entities.WeekNavigation{
			Current: weeks[0],
			Prev:    weeks[1],
		}, nil
	}

	weekStr := week.Format("2006-01-02")
	var currentStr, prevStr, nextStr string

	if err := r.getWeekStr(ctx, "DESC", squirrel.And{squirrel.Eq{"name": name}, squirrel.Eq{"week": weekStr}}, &currentStr); err != nil {
		return entities.WeekNavigation{}, fmt.Errorf("%s: get current week: %w", op, err)
	}
	current, _ := time.Parse("2006-01-02", currentStr)

	var prev, next time.Time
	if err := r.getWeekStr(ctx, "DESC", squirrel.And{squirrel.Eq{"name": name}, squirrel.Lt{"week": weekStr}}, &prevStr); err == nil {
		prev, _ = time.Parse("2006-01-02", prevStr)
	}

	if err := r.getWeekStr(ctx, "ASC", squirrel.And{squirrel.Eq{"name": name}, squirrel.Gt{"week": weekStr}}, &nextStr); err == nil {
		next, _ = time.Parse("2006-01-02", nextStr)
	}

	return entities.WeekNavigation{
		Prev:    prev,
		Current: current,
		Next:    next,
	}, nil
}

func (r *TeacherScheduleRepository) GetAllTeacherNames(ctx context.Context) ([]string, error) {
	const op = "repository.TeacherScheduleRepository.GetAllTeacherNames"

	query := r.sb.Select("DISTINCT name").
		From("teacher_schedules").
		OrderBy("name ASC")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: query teacher names: %w", op, err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("%s: scan teacher name: %w", op, err)
		}
		names = append(names, name)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows err: %w", op, err)
	}

	return names, nil
}

func (r *TeacherScheduleRepository) GetAllLatest(ctx context.Context) (map[string]*entities.Teacher, error) {
	const op = "repository.TeacherScheduleRepository.GetAllLatest"

	// Чистый SQLite запрос для получения последнего расписания каждого преподавателя
	query := `
		SELECT ts.name, ts.schedule 
		FROM teacher_schedules ts
		INNER JOIN (
			SELECT name, MAX(week) AS max_week 
			FROM teacher_schedules 
			GROUP BY name
		) latest ON ts.name = latest.name AND ts.week = latest.max_week
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%s: query all latest: %w", op, err)
	}
	defer rows.Close()

	result := make(map[string]*entities.Teacher)
	for rows.Next() {
		var name string
		var raw string
		if err := rows.Scan(&name, &raw); err != nil {
			return nil, fmt.Errorf("%s: scan row: %w", op, err)
		}

		var t entities.Teacher
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			return nil, fmt.Errorf("%s: unmarshal teacher: %w", op, err)
		}

		result[name] = &t
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows err: %w", op, err)
	}

	return result, nil
}

func (r *TeacherScheduleRepository) GetAll(ctx context.Context) ([]*entities.Teacher, error) {
	const op = "repository.TeacherScheduleRepository.GetAll"

	query := r.sb.Select("schedule").From("teacher_schedules")
	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build query: %w", op, err)
	}

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("%s: query all: %w", op, err)
	}
	defer rows.Close()

	var result []*entities.Teacher
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("%s: scan row: %w", op, err)
		}

		var t entities.Teacher
		if err := json.Unmarshal([]byte(raw), &t); err != nil {
			continue
		}

		result = append(result, &t)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s: rows err: %w", op, err)
	}

	return result, nil
}

func (r *TeacherScheduleRepository) getWeekStr(ctx context.Context, orderBy string, pred, dest any) error {
	query := r.sb.Select("week").
		From("teacher_schedules").
		Where(pred).
		OrderBy("week " + orderBy).
		Limit(1)
	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return err
	}

	return r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(dest)
}
