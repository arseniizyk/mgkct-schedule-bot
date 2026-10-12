package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
)

type ScheduleRepository struct {
	pool *pgxpool.Pool
	sb   squirrel.StatementBuilderType
}

func NewScheduleRepository(pool *pgxpool.Pool) *ScheduleRepository {
	return &ScheduleRepository{
		pool: pool,
		sb:   squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (r *ScheduleRepository) Save(ctx context.Context, week time.Time, schedule *entities.Schedule) error {
	const op = "repository.ScheduleRepository.Save"

	data, err := json.Marshal(schedule)
	if err != nil {
		return fmt.Errorf("%s: marshal schedule: %w", op, err)
	}

	query := r.sb.Insert("schedules").
		Columns("week", "schedule").
		Values(week, data).
		Suffix("ON CONFLICT (week) DO UPDATE SET schedule = EXCLUDED.schedule")

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	if _, err := r.pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("%s: exec sql query: %w", op, err)
	}

	return nil
}

func (r *ScheduleRepository) GetByWeek(ctx context.Context, week time.Time) (*entities.Schedule, error) {
	const op = "repository.ScheduleRepository.GetByWeek"

	query := r.sb.Select("schedule").
		From("schedules").
		Where(squirrel.LtOrEq{"week": week}).
		OrderBy("week DESC").
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	var raw []byte
	if err := r.pool.QueryRow(ctx, sql, args...).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.ErrWeekNotFound
		}
		return nil, fmt.Errorf("%s: get schedule: %w", op, err)
	}

	var s entities.Schedule
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("%s: unmarshal schedule: %w", op, err)
	}

	return &s, nil
}

func (r *ScheduleRepository) GetLatest(ctx context.Context) (*entities.Schedule, error) {
	const op = "repository.ScheduleRepository.GetLatest"

	query := r.sb.Select("schedule").
		From("schedules").
		OrderBy("updated_at DESC").
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	var raw []byte
	if err := r.pool.QueryRow(ctx, sql, args...).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domainerr.ErrScheduleNotFound
		}
		return nil, fmt.Errorf("%s: get latest schedule: %w", op, err)
	}

	var s entities.Schedule
	if err := json.Unmarshal(raw, &s); err != nil {
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

		sql, args, err := query.ToSql()
		if err != nil {
			return entities.WeekNavigation{}, fmt.Errorf("%s: failed to create query: %w", op, err)
		}

		rows, err := r.pool.Query(ctx, sql, args...)
		if err != nil {
			return entities.WeekNavigation{}, fmt.Errorf("%s: repository sql query: %w", op, err)
		}
		defer rows.Close()

		var weeks []time.Time
		for rows.Next() {
			var w time.Time
			if err := rows.Scan(&w); err != nil {
				return entities.WeekNavigation{}, fmt.Errorf("%s: repository scan rows: %w", op, err)
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

	var current, prev, next time.Time

	if err := r.getWeek(ctx, "DESC", squirrel.Eq{"week": week}, &current); err != nil {
		return entities.WeekNavigation{}, fmt.Errorf("%s: get current week: %w", op, err)
	}

	if err := r.getWeek(ctx, "DESC", squirrel.Lt{"week": week}, &prev); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return entities.WeekNavigation{}, fmt.Errorf("%s: get prev week: %w", op, err)
		}
		prev = time.Time{}
	}

	if err := r.getWeek(ctx, "ASC", squirrel.Gt{"week": week}, &next); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return entities.WeekNavigation{}, fmt.Errorf("%s: get next week: %w", op, err)
		}
		next = time.Time{}
	}

	return entities.WeekNavigation{
		Prev:    prev,
		Current: current,
		Next:    next,
	}, nil
}

func (r *ScheduleRepository) getWeek(ctx context.Context, orderBy string, pred, dest any) error {
	query := r.sb.Select("week").
		From("schedules").
		Where(pred).
		OrderBy("week " + orderBy).
		Limit(1)
	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	row := r.pool.QueryRow(ctx, sql, args...)
	if err := row.Scan(dest); err != nil {
		return err
	}

	return nil
}
