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

type ScheduleRepository struct {
	db *sql.DB
	sb squirrel.StatementBuilderType
}

func NewScheduleRepository(db *sql.DB) *ScheduleRepository {
	return &ScheduleRepository{
		db: db,
		sb: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question),
	}
}

func (r *ScheduleRepository) Save(ctx context.Context, week time.Time, schedule *entities.Schedule) error {
	const op = "repository.ScheduleRepository.Save"

	data, err := json.Marshal(schedule)
	if err != nil {
		return fmt.Errorf("%s: marshal schedule: %w", op, err)
	}

	weekStr := week.Format("2006-01-02")
	query := r.sb.Insert("schedules").
		Columns("week", "schedule").
		Values(weekStr, string(data)).
		Suffix("ON CONFLICT(week) DO UPDATE SET schedule = excluded.schedule, updated_at = CURRENT_TIMESTAMP")

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	if _, err := r.db.ExecContext(ctx, sqlQuery, args...); err != nil {
		return fmt.Errorf("%s: exec sql query: %w", op, err)
	}

	return nil
}

func (r *ScheduleRepository) GetByWeek(ctx context.Context, week time.Time) (*entities.Schedule, error) {
	const op = "repository.ScheduleRepository.GetByWeek"

	weekStr := week.Format("2006-01-02")
	query := r.sb.Select("schedule").
		From("schedules").
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
		return nil, fmt.Errorf("%s: get schedule: %w", op, err)
	}

	var s entities.Schedule
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("%s: unmarshal schedule: %w", op, err)
	}

	return &s, nil
}

func (r *ScheduleRepository) GetLatest(ctx context.Context) (*entities.Schedule, error) {
	const op = "repository.ScheduleRepository.GetLatest"

	query := r.sb.Select("schedule").
		From("schedules").
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
		return nil, fmt.Errorf("%s: get latest schedule: %w", op, err)
	}

	var s entities.Schedule
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, fmt.Errorf("%s: unmarshal schedule: %w", op, err)
	}

	return &s, nil
}

func (r *ScheduleRepository) GetWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error) {
	const op = "repository.ScheduleRepository.GetWeeks"

	week = time.Date(week.Year(), week.Month(), week.Day(), 0, 0, 0, 0, time.UTC)

	if week.IsZero() {
		query := r.sb.Select("week").
			From("schedules").
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
				return entities.WeekNavigation{}, fmt.Errorf("%s: scan row: %w", op, err)
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

	if err := r.getWeekStr(ctx, "DESC", squirrel.Eq{"week": weekStr}, &currentStr); err != nil {
		return entities.WeekNavigation{}, fmt.Errorf("%s: get current week: %w", op, err)
	}
	current, _ := time.Parse("2006-01-02", currentStr)

	var prev, next time.Time
	if err := r.getWeekStr(ctx, "DESC", squirrel.Lt{"week": weekStr}, &prevStr); err == nil {
		prev, _ = time.Parse("2006-01-02", prevStr)
	}

	if err := r.getWeekStr(ctx, "ASC", squirrel.Gt{"week": weekStr}, &nextStr); err == nil {
		next, _ = time.Parse("2006-01-02", nextStr)
	}

	return entities.WeekNavigation{
		Prev:    prev,
		Current: current,
		Next:    next,
	}, nil
}

func (r *ScheduleRepository) getWeekStr(ctx context.Context, orderBy string, pred, dest any) error {
	query := r.sb.Select("week").
		From("schedules").
		Where(pred).
		OrderBy("week " + orderBy).
		Limit(1)
	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return err
	}

	return r.db.QueryRowContext(ctx, sqlQuery, args...).Scan(dest)
}
