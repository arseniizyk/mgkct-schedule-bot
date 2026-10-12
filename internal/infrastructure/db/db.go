package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Connect(ctx context.Context, cfg *config.DBConfig) (*pgxpool.Pool, error) {
	dsn := cfg.DSN()

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("db: connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping database: %w", err)
	}

	if err := migrateUp(ctx, dsn); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: apply migrations: %w", err)
	}

	return pool, nil
}

func migrateUp(ctx context.Context, dsn string) error {
	g, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("db: open migration connection: %w", err)
	}
	defer func() { _ = g.Close() }()

	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("db: set goose dialect: %w", err)
	}

	if err := goose.UpContext(ctx, g, "migrations"); err != nil {
		return fmt.Errorf("db: apply up migrations: %w", err)
	}

	return nil
}
