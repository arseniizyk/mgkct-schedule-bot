package repository_test

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupFacultyDB(t *testing.T) (*sql.DB, *repository.FacultyRepository) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE faculty (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		full_name TEXT NOT NULL,
		short_name TEXT NOT NULL,
		position TEXT NOT NULL DEFAULT '',
		qualification TEXT NOT NULL DEFAULT '',
		education TEXT NOT NULL DEFAULT '',
		experience TEXT NOT NULL DEFAULT '',
		department TEXT NOT NULL DEFAULT '',
		full_name_lower TEXT NOT NULL DEFAULT '',
		short_name_lower TEXT NOT NULL DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(full_name, department)
	);`)
	require.NoError(t, err)

	repo := repository.NewFacultyRepository(db)
	return db, repo
}

func TestFacultyRepository_SaveBatchAndCyrillicSearch(t *testing.T) {
	db, repo := setupFacultyDB(t)
	defer db.Close()
	ctx := context.Background()

	members := []*entities.FacultyMember{
		{
			FullName:   "Иванов Иван Иванович",
			ShortName:  "Иванов И. И.",
			Position:   "Преподаватель",
			Department: "Отделение информационных технологий",
		},
		{
			FullName:   "Петров Алексей Сергеевич",
			ShortName:  "Петров А. С.",
			Position:   "Мастер ПО",
			Department: "Отделение радиотехники",
		},
		{
			FullName:   "Смирнов Сергей Сергеевич",
			ShortName:  "Смирнов С. С.",
			Position:   "Преподаватель",
			Department: "Общеобразовательные дисциплины",
		},
	}

	err := repo.SaveBatch(ctx, members)
	require.NoError(t, err)

	// Test 1: Lowercase Cyrillic search
	res, err := repo.Search(ctx, "иван")
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "Иванов Иван Иванович", res[0].FullName)

	// Test 2: Uppercase Cyrillic search
	res, err = repo.Search(ctx, "ИВАНОВ")
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "Иванов Иван Иванович", res[0].FullName)

	// Test 3: Mixed-case Cyrillic search
	res, err = repo.Search(ctx, "пЕтРоВ")
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "Петров Алексей Сергеевич", res[0].FullName)

	// Test 4: Search by short name initials
	res, err = repo.Search(ctx, "смирнов с.")
	require.NoError(t, err)
	require.Len(t, res, 1)
	assert.Equal(t, "Смирнов Сергей Сергеевич", res[0].FullName)

	// Test 5: Search non-existent
	res, err = repo.Search(ctx, "Сидоров")
	require.NoError(t, err)
	assert.Empty(t, res)
}
