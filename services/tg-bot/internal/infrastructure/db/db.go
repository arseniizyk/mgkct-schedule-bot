package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"net"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // драйвер database/sql для goose
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Connect(ctx context.Context, cfg *config.PostgresConfig) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s/%s?sslmode=disable",
		cfg.User,
		cfg.Password,
		net.JoinHostPort(cfg.Host, cfg.Port),
		cfg.DBName,
	)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("db: connect to database: url: %s, err: %w", dsn, err)
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
		return fmt.Errorf("open database connection: %w", err)
	}
	defer func() { _ = g.Close() }()

	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}

	if err := goose.UpContext(ctx, g, "migrations"); err != nil {
		return fmt.Errorf("apply up migrations: %w", err)
	}

	return nil
}
