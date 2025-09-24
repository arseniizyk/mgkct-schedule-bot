package database

import (
	"context"
	"fmt"
	"os"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Database struct {
	Pool *pgxpool.Pool
}

func New(cfg *config.Config) (*Database, error) {
	url := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		os.Getenv("POSTGRES_HOST"),
		cfg.PostgresPort,
		cfg.PostgresDB,
		cfg.PostgresSSL,
	)

	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		return nil, fmt.Errorf("db: connect to database: url: %s, err: %w", url, err)
	}

	return &Database{
		Pool: pool,
	}, nil
}

func (d *Database) Close() {
	d.Pool.Close()
}

func (d *Database) Ping(ctx context.Context) error {
	return d.Pool.Ping(ctx)
}
