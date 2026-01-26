package database

import (
	"context"
	"fmt"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(cfg *config.PostgresConfig) (*pgxpool.Pool, error) {
	url := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
	)

	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		return nil, fmt.Errorf("db: connect to database: url: %s, err: %w", url, err)
	}

	return pool, nil
}
