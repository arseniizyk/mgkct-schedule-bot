package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Masterminds/squirrel"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
)

type Schedule interface {
	Save(ctx context.Context, week time.Time, schedule *pb.Schedule) error
	GetByWeek(ctx context.Context, week time.Time) (*pb.Schedule, error)
	GetLatest(ctx context.Context) (*pb.Schedule, error)
	GetWeeks(ctx context.Context, week time.Time) (*model.Weeks, error)
}

type repository struct {
	pool *pgxpool.Pool
	sb   squirrel.StatementBuilderType
}

func New(pool *pgxpool.Pool) Schedule {
	return &repository{
		pool: pool,
		sb:   squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (repo *repository) Save(ctx context.Context, week time.Time, schedule *pb.Schedule) error {
	data, err := protojson.Marshal(schedule)
	if err != nil {
		return fmt.Errorf("repo: marshal schedule: %w", err)
	}

	query := repo.sb.Insert("schedules").
		Columns("week", "schedule").
		Values(week, data).Suffix("ON CONFLICT (week) DO UPDATE SET schedule = EXCLUDED.schedule")

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("repo: build query to sql: %w", err)
	}

	_, err = repo.pool.Exec(ctx, sql, args...)
	return err
}

func (repo *repository) GetByWeek(ctx context.Context, week time.Time) (*pb.Schedule, error) {
	query := repo.sb.Select("schedule").
		From("schedules").
		Where(squirrel.LtOrEq{"week": week}).
		OrderBy("week DESC").
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("repo: build query to sql: %w", err)
	}

	var raw []byte
	if err := repo.pool.QueryRow(ctx, sql, args...).Scan(&raw); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("repo: get schedule: %w", err)
	}

	var s pb.Schedule
	if err := protojson.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("repo: unmarshal schedule: %w", err)
	}

	return &s, nil
}

func (repo *repository) GetLatest(ctx context.Context) (*pb.Schedule, error) {
	query := repo.sb.Select("schedule").
		From("schedules").
		OrderBy("updated_at DESC").
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("repo: build query to sql: %w", err)
	}

	var raw []byte
	err = repo.pool.QueryRow(ctx, sql, args...).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, model.ErrNotFound
		}
		return nil, fmt.Errorf("repo: get latest schedule: %w", err)
	}

	var s pb.Schedule
	if err := protojson.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("repo: unmarshal schedule: %w", err)
	}

	return &s, nil
}

func (repo *repository) GetWeeks(ctx context.Context, week time.Time) (*model.Weeks, error) {
	week = time.Date(week.Year(), week.Month(), week.Day(), 0, 0, 0, 0, time.UTC)

	if week.IsZero() {
		query := repo.sb.Select("week").
			From("schedules").
			OrderBy("week DESC").
			Limit(2)

		sql, args, _ := query.ToSql()

		rows, err := repo.pool.Query(ctx, sql, args...)
		if err != nil {
			return nil, fmt.Errorf("repository sql query: %w", err)
		}
		defer rows.Close()

		var weeks []time.Time

		for rows.Next() {
			var w time.Time
			if err := rows.Scan(&w); err != nil {
				return nil, fmt.Errorf("repository scan rows: %w", err)
			}
			weeks = append(weeks, w)
		}

		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("sql rows err: %w", err)
		}

		if len(weeks) < 2 {
			slog.Debug("Not enough weeks in db")
			return nil, fmt.Errorf("not enough weeks")
		}

		return &model.Weeks{
			Current: weeks[0],
			Prev:    weeks[1],
		}, nil
	}

	var current, prev, next time.Time

	if err := repo.getWeek(ctx, squirrel.Eq{"week": week}, &current); err != nil {
		return nil, fmt.Errorf("repository get current week: %w", err)
	}

	if err := repo.getWeek(ctx, squirrel.Lt{"week": week}, &prev); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("repository get prev week: %w", err)
		}
		// if user has reached the edge, so we return nil as prev
		prev = time.Time{}
	}

	if err := repo.getWeek(ctx, squirrel.Gt{"week": week}, &next); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("repository get next week: %w", err)
		}
		// if user has reached the edge, so we return nil as next
		next = time.Time{}
	}

	return &model.Weeks{
		Prev:    prev,
		Current: current,
		Next:    next,
	}, nil
}

func (repo *repository) getWeek(ctx context.Context, pred any, dest any) error {
	query := repo.sb.Select("week").
		From("schedules").
		Where(pred).
		OrderBy("week DESC").
		Limit(1)
	sql, args, _ := query.ToSql()

	row := repo.pool.QueryRow(ctx, sql, args...)
	if err := row.Scan(dest); err != nil {
		return err
	}

	return nil
}
