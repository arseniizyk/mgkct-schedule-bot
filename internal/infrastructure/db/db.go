package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Connect(ctx context.Context, cfg *config.DBConfig) (*sql.DB, error) {
	// Создаем родительскую директорию для файла БД при необходимости
	dir := filepath.Dir(cfg.Path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("db: create directory for sqlite: %w", err)
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", cfg.Path)

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("db: open sqlite database: %w", err)
	}

	// Настройка пула для SQLite: в режиме WAL несколько читателей, 1 писатель
	database.SetMaxOpenConns(10)
	database.SetMaxIdleConns(5)

	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("db: ping sqlite database: %w", err)
	}

	if err := migrateUp(ctx, database); err != nil {
		_ = database.Close()
		return nil, fmt.Errorf("db: apply migrations: %w", err)
	}

	return database, nil
}

func migrateUp(ctx context.Context, database *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	goose.SetLogger(goose.NopLogger())

	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("db: set goose dialect: %w", err)
	}

	if err := goose.UpContext(ctx, database, "migrations"); err != nil {
		return fmt.Errorf("db: apply up migrations: %w", err)
	}

	return nil
}
