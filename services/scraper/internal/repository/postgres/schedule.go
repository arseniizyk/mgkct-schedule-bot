package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/repository"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
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

func (repo *ScheduleRepository) Save(ctx context.Context, week time.Time, schedule *pb.Schedule) error {
	const op = "repository.postgres.ScheduleRepository.Save"

	data, err := protojson.Marshal(schedule)
	if err != nil {
		return fmt.Errorf("%s: marshal schedule: %w", op, err)
	}

	query := repo.sb.Insert("schedules").
		Columns("week", "schedule").
		Values(week, data).
		Suffix("ON CONFLICT (week) DO UPDATE SET schedule = EXCLUDED.schedule")

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	if _, err := repo.pool.Exec(ctx, sql, args...); err != nil {
		return fmt.Errorf("%s: exec sql query: %w", op, err)
	}

	return nil
}

func (repo *ScheduleRepository) GetByWeek(ctx context.Context, week time.Time) (*pb.Schedule, error) {
	const op = "repository.postgres.ScheduleRepository.GetByWeek"

	query := repo.sb.Select("schedule").
		From("schedules").
		Where(squirrel.LtOrEq{"week": week}).
		OrderBy("week DESC").
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	var raw []byte
	if err := repo.pool.QueryRow(ctx, sql, args...).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrWeekNotFound
		}
		return nil, fmt.Errorf("%s: get schedule: %w", op, err)
	}

	var s pb.Schedule
	if err := protojson.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("%s: unmarshal schedule: %w", op, err)
	}

	return &s, nil
}

func (repo *ScheduleRepository) GetLatest(ctx context.Context) (*pb.Schedule, error) {
	const op = "repository.postgres.ScheduleRepository.GetLatest"

	query := repo.sb.Select("schedule").
		From("schedules").
		OrderBy("updated_at DESC").
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("%s: build query to sql: %w", op, err)
	}

	var raw []byte
	err = repo.pool.QueryRow(ctx, sql, args...).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, repository.ErrScheduleNotFound
		}
		return nil, fmt.Errorf("%s: get latest schedule: %w", op, err)
	}

	var s pb.Schedule
	if err := protojson.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("%s: unmarshal schedule: %w", op, err)
	}

	return &s, nil
}

func (repo *ScheduleRepository) GetWeeks(ctx context.Context, week time.Time) (entities.WeekNavigation, error) {
	const op = "repository.postgres.ScheduleRepository.GetWeeks"

	week = time.Date(week.Year(), week.Month(), week.Day(), 0, 0, 0, 0, time.UTC)

	if week.IsZero() {
		query := repo.sb.Select("week").
			From("schedules").
			OrderBy("week DESC").
			Limit(2)

		sql, args, err := query.ToSql()
		if err != nil {
			return entities.WeekNavigation{}, fmt.Errorf("%s: failed to create query: %w", op, err)
		}

		rows, err := repo.pool.Query(ctx, sql, args...)
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
			return entities.WeekNavigation{}, repository.ErrNoAvailableWeeks
		}

		return entities.WeekNavigation{
			Current: weeks[0],
			Prev:    weeks[1],
		}, nil
	}

	var current, prev, next time.Time

	if err := repo.getWeek(ctx, "DESC", squirrel.Eq{"week": week}, &current); err != nil {
		return entities.WeekNavigation{}, fmt.Errorf("%s: get current week: %w", op, err)
	}

	if err := repo.getWeek(ctx, "DESC", squirrel.Lt{"week": week}, &prev); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return entities.WeekNavigation{}, fmt.Errorf("%s: get prev week: %w", op, err)
		}
		// if user has reached the edge, so we return nil as prev
		prev = time.Time{}
	}

	if err := repo.getWeek(ctx, "ASC", squirrel.Gt{"week": week}, &next); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return entities.WeekNavigation{}, fmt.Errorf("%s: get next week: %w", op, err)
		}
		// if user has reached the edge, so we return nil as next
		next = time.Time{}
	}

	return entities.WeekNavigation{
		Prev:    prev,
		Current: current,
		Next:    next,
	}, nil
}

func (repo *ScheduleRepository) getWeek(ctx context.Context, orderBy string, pred, dest any) error {
	query := repo.sb.Select("week").
		From("schedules").
		Where(pred).
		OrderBy("week " + orderBy).
		Limit(1)
	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	row := repo.pool.QueryRow(ctx, sql, args...)
	if err := row.Scan(dest); err != nil {
		return err
	}

	return nil
}
