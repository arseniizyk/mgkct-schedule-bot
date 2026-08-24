package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var configPath, migrationsPath, command string
	flag.StringVar(&configPath, "config_path", "", "path to config")
	flag.StringVar(&migrationsPath, "migrations_path", "", "path to migrations")
	flag.StringVar(&command, "command", "up", "command for migrations(up ,down)")
	flag.Parse()

	cfg, err := config.New(configPath)
	if err != nil {
		log.Fatalf("failed to get config in migrations: Path: %s, error: %s", configPath, err)
	}

	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.Scraper.DB.User,
		cfg.Scraper.DB.Password,
		cfg.Scraper.DB.Host,
		cfg.Scraper.DB.Port,
		cfg.Scraper.DB.DBName,
	)

	m, err := migrate.New(fmt.Sprintf("file://%s", migrationsPath), dsn)
	if err != nil {
		log.Fatalf("failed to migrate: Path: %s, error: %s", migrationsPath, err)
	}
	defer func() { _, _ = m.Close() }()

	switch command {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("failed to apply up migration: %s", err)
		}
	case "down":
		if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("failed to apply down migration: %s", err)
		}
	case "drop":
		if err := m.Drop(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("failed to drop migration: %s", err)
		}
	}

	slog.Info("Migrations successfully applied!")
}
