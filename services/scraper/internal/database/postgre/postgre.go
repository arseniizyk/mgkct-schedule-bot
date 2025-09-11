package postgre

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/database"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	pool *pgxpool.Pool
	sb   squirrel.StatementBuilderType
}

var (
	ErrCantConnect = errors.New("can't connect to database url")
)

func NewDatabase(url string) (database.DatabaseRepository, error) {
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		return nil, ErrCantConnect
	}

	return &Database{
		pool: pool,
		sb:   squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}, nil
}

func (d *Database) SaveSchedule(ctx context.Context, week time.Time, schedule *models.Schedule) error {
	data, err := json.Marshal(schedule)
	if err != nil {
		return fmt.Errorf("failed to marshal schedule: %w", err)
	}

	query := d.sb.Insert("schedules").
		Columns("week", "schedule").
		Values(week, data)

	sql, args, err := query.ToSql()
	if err != nil {
		return err
	}

	_, err = d.pool.Exec(ctx, sql, args...)
	return err
}

func (d *Database) GetSchedule(ctx context.Context, week time.Time) (*models.Schedule, error) {
	query := d.sb.Select("schedule").
		From("schedules").
		Where(squirrel.Eq{"week": week})
	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't parse query to SQL: %w", err)
	}

	var raw []byte
	err = d.pool.QueryRow(ctx, sql, args...).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("can't get schedule: %w", err)
	}

	var s models.Schedule
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schedule: %w", err)
	}

	return &s, nil
}

func (d *Database) GetLatestSchedule(ctx context.Context) (*models.Schedule, error) {
	query := d.sb.Select("schedule").
		From("schedules").
		OrderBy("week DESC").
		Limit(1)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, fmt.Errorf("can't parse query to SQL: %w", err)
	}

	var raw []byte
	err = d.pool.QueryRow(ctx, sql, args...).Scan(&raw)
	if err != nil {
		return nil, fmt.Errorf("can't get latest schedule: %w", err)
	}

	var s models.Schedule
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("failed to unmarshal schedule: %w", err)
	}

	return &s, nil
}

func (d *Database) Close() {
	d.pool.Close()
}

func (d *Database) Ping(ctx context.Context) error {
	return d.pool.Ping(ctx)
}
