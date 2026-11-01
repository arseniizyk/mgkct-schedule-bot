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

func setupTeacherScheduleDB(t *testing.T) (*sql.DB, *repository.TeacherScheduleRepository) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE teacher_schedules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		week TEXT NOT NULL,
		schedule TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(name, week)
	);`)
	require.NoError(t, err)

	repo := repository.NewTeacherScheduleRepository(db)
	return db, repo
}

func TestTeacherScheduleRepository_GetByWeek_StrictMatch(t *testing.T) {
	db, repo := setupTeacherScheduleDB(t)
	defer db.Close()
	ctx := context.Background()

	const teacher = "Иванов И. И."

	// Insert existing schedule for 2026-11-02
	_, err := db.Exec(`INSERT INTO teacher_schedules (name, week, schedule) VALUES (?, '2026-11-02', '{"name":"Иванов И. И.","days":[]}')`, teacher)
	require.NoError(t, err)

	// 1. Existing week should return successfully
	existingDate, _ := time.Parse("2006-01-02", "2026-11-02")
	sched, err := repo.GetByWeek(ctx, teacher, existingDate)
	require.NoError(t, err)
	assert.NotNil(t, sched)
	assert.Equal(t, teacher, sched.Name)

	// 2. Future week (2026-11-09) must return ErrWeekNotFound (not 2026-11-02!)
	futureDate, _ := time.Parse("2006-01-02", "2026-11-09")
	schedFuture, err := repo.GetByWeek(ctx, teacher, futureDate)
	require.ErrorIs(t, err, domainerr.ErrWeekNotFound)
	assert.Nil(t, schedFuture)
}

func TestTeacherScheduleRepository_GetWeeks_ZeroTime(t *testing.T) {
	db, repo := setupTeacherScheduleDB(t)
	defer db.Close()
	ctx := context.Background()

	const teacher = "Иванов И. И."

	_, err := db.Exec(`INSERT INTO teacher_schedules (name, week, schedule) VALUES (?, '2026-11-02', '{"name":"Иванов И. И."}')`, teacher)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO teacher_schedules (name, week, schedule) VALUES (?, '2026-10-26', '{"name":"Иванов И. И."}')`, teacher)
	require.NoError(t, err)

	// Passing zero time should return latest 2 weeks
	nav, err := repo.GetWeeks(ctx, teacher, time.Time{})
	require.NoError(t, err)
	assert.Equal(t, "2026-11-02", nav.Current.Format("2006-01-02"))
	assert.Equal(t, "2026-10-26", nav.Prev.Format("2006-01-02"))
}
