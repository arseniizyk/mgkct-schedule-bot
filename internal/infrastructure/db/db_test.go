package db_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/infrastructure/db"
)

func TestConnect_MigrationsAndTriggers(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	cfg := &config.DBConfig{
		Path: dbPath,
	}

	ctx := context.Background()
	database, err := db.Connect(ctx, cfg)
	require.NoError(t, err)
	defer database.Close()

	// Verify schedules table trigger
	_, err = database.ExecContext(ctx, `INSERT INTO schedules (week, schedule) VALUES ('2026-11-02', '{}')`)
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `UPDATE schedules SET schedule = '{"updated":true}' WHERE week = '2026-11-02'`)
	require.NoError(t, err)

	// Verify teacher_schedules table trigger
	_, err = database.ExecContext(ctx, `INSERT INTO teacher_schedules (name, week, schedule) VALUES ('Иванов', '2026-11-02', '{}')`)
	require.NoError(t, err)

	_, err = database.ExecContext(ctx, `UPDATE teacher_schedules SET schedule = '{"updated":true}' WHERE name = 'Иванов' AND week = '2026-11-02'`)
	require.NoError(t, err)

	// Verify faculty columns (full_name_lower, short_name_lower)
	_, err = database.ExecContext(ctx, `INSERT INTO faculty (full_name, short_name, full_name_lower, short_name_lower) VALUES ('Тест', 'Т.', 'тест', 'т.')`)
	require.NoError(t, err)

	// Verify faculty trigger
	_, err = database.ExecContext(ctx, `UPDATE faculty SET position = 'Преподаватель' WHERE full_name = 'Тест'`)
	require.NoError(t, err)
}
