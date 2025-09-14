package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/schedule/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ScheduleRepository struct {
	pool *pgxpool.Pool
	sb   squirrel.StatementBuilderType
}

func New(pool *pgxpool.Pool) repository.ScheduleRepository {
	return &ScheduleRepository{
		pool: pool,
		sb:   squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}
}

func (repo *ScheduleRepository) Save(ctx context.Context, week time.Time, schedule *models.Schedule) error {
	data, err := json.Marshal(schedule)
	if err != nil {
		return fmt.Errorf("repo: marshal schedule: %w", err)
	}

	query := repo.sb.Insert("schedules").
		Columns("week", "schedule").
		Values(week, data)

	sql, args, err := query.ToSql()
	if err != nil {
		return fmt.Errorf("repo: build query to sql: %w", err)
	}

	_, err = repo.pool.Exec(ctx, sql, args...)
	return err
}

func (repo *ScheduleRepository) GetByWeek(ctx context.Context, week time.Time) (*models.Schedule, error) {
	query := repo.sb.Select("schedule").
		From("schedules").
		Where(squirrel.Eq{"week": week})

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("repo: build query to sql: %w", err)
	}

	var raw []byte
	err = repo.pool.QueryRow(ctx, sql, args...).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("repo: get schedule: %w", err)
	}

	var s models.Schedule
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("repo: unmarshal schedule: %w", err)
	}

	return &s, nil
}

func (repo *ScheduleRepository) GetLatest(ctx context.Context) (*models.Schedule, error) {
	query := repo.sb.Select("schedule").
		From("schedules").
		OrderBy("updated DESC").
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("repo: build query to sql: %w", err)
	}

	var raw []byte
	err = repo.pool.QueryRow(ctx, sql, args...).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("repo: get latest schedule: %w", err)
	}

	var s models.Schedule
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("repo: unmarshal schedule: %w", err)
	}

	return &s, nil
}
