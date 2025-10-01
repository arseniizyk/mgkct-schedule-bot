package schedule

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	pb "github.com/arseniizyk/mgkct-schedule-bot/libs/proto"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/schedule/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/encoding/protojson"
)

type repository struct {
	pool *pgxpool.Pool
	sb   squirrel.StatementBuilderType
}

func New(pool *pgxpool.Pool) *repository {
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
