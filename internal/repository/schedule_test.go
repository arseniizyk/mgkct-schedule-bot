package repository_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupScheduleDB(t *testing.T) (*sql.DB, *repository.ScheduleRepository) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE schedules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		week TEXT UNIQUE NOT NULL,
		schedule TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`)
	require.NoError(t, err)

	repo := repository.NewScheduleRepository(db)
	return db, repo
}

func TestScheduleRepository_GetByWeek_StrictMatch(t *testing.T) {
	db, repo := setupScheduleDB(t)
	defer db.Close()
	ctx := context.Background()

	// Insert existing schedule for 2026-11-02
	_, err := db.Exec(`INSERT INTO schedules (week, schedule) VALUES ('2026-11-02', '{"week":"2026-11-02"}')`)
	require.NoError(t, err)

	// 1. Existing week should return successfully
	existingDate, _ := time.Parse("2006-01-02", "2026-11-02")
	sched, err := repo.GetByWeek(ctx, existingDate)
	require.NoError(t, err)
	assert.NotNil(t, sched)

	// 2. Future week (2026-11-09) must return ErrWeekNotFound (not 2026-11-02!)
	futureDate, _ := time.Parse("2006-01-02", "2026-11-09")
	schedFuture, err := repo.GetByWeek(ctx, futureDate)
	assert.ErrorIs(t, err, domainerr.ErrWeekNotFound)
	assert.Nil(t, schedFuture)
}

func TestScheduleRepository_GetWeeks_ZeroTime(t *testing.T) {
	db, repo := setupScheduleDB(t)
	defer db.Close()
	ctx := context.Background()

	_, err := db.Exec(`INSERT INTO schedules (week, schedule) VALUES ('2026-11-02', '{"week":"2026-11-02"}')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO schedules (week, schedule) VALUES ('2026-10-26', '{"week":"2026-10-26"}')`)
	require.NoError(t, err)

	// Passing zero time should return latest 2 weeks
	nav, err := repo.GetWeeks(ctx, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, "2026-11-02", nav.Current.Format("2006-01-02"))
	assert.Equal(t, "2026-10-26", nav.Prev.Format("2006-01-02"))
}
