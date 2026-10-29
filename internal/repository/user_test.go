package repository_test

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/internal/domain/errors"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*sql.DB, *repository.UserRepository) {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		chat_id INTEGER UNIQUE NOT NULL,
		username TEXT,
		group_id INTEGER,
		subgroup INTEGER NOT NULL DEFAULT 0,
		teacher_name TEXT,
		state TEXT NOT NULL DEFAULT '',
		notify_evening INTEGER NOT NULL DEFAULT 1
	);`)
	require.NoError(t, err)

	repo := repository.NewUserRepository(db)
	return db, repo
}

func TestUserRepository_SetUserGroup_WithSubgroup(t *testing.T) {
	db, repo := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Сохраняем группу 101 с 1-й подгруппой
	err := repo.SetUserGroup(ctx, 12345, 101, 1)
	require.NoError(t, err)

	grp, err := repo.GroupByChatID(ctx, 12345)
	require.NoError(t, err)
	assert.Equal(t, 101, grp)

	sg, err := repo.SubgroupByChatID(ctx, 12345)
	require.NoError(t, err)
	assert.Equal(t, 1, sg)

	grp, sg, err = repo.GroupAndSubgroupByChatID(ctx, 12345)
	require.NoError(t, err)
	assert.Equal(t, 101, grp)
	assert.Equal(t, 1, sg)

	// 2. Меняем на 2-ю подгруппу
	err = repo.SetUserSubgroup(ctx, 12345, 2)
	require.NoError(t, err)

	sg, err = repo.SubgroupByChatID(ctx, 12345)
	require.NoError(t, err)
	assert.Equal(t, 2, sg)

	// 3. Установка преподавателя должна сбросить группу и подгруппу
	err = repo.SetTeacher(ctx, 12345, "Иванов И.И.")
	require.NoError(t, err)

	_, err = repo.GroupByChatID(ctx, 12345)
	require.ErrorIs(t, err, domainerr.ErrUserNoGroup)

	sg, err = repo.SubgroupByChatID(ctx, 12345)
	require.NoError(t, err)
	assert.Equal(t, 0, sg)

	teacher, err := repo.GetTeacher(ctx, 12345)
	require.NoError(t, err)
	assert.Equal(t, "Иванов И.И.", teacher)
}

func TestUserRepository_AllUsers_AndEveningNotification(t *testing.T) {
	db, repo := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	err := repo.SaveUser(ctx, entities.User{ChatID: 1001, Username: "student1"})
	require.NoError(t, err)
	err = repo.SetUserGroup(ctx, 1001, 101, 1)
	require.NoError(t, err)

	err = repo.SaveUser(ctx, entities.User{ChatID: 1002, Username: "student2"})
	require.NoError(t, err)
	err = repo.SetUserGroup(ctx, 1002, 101, 2)
	require.NoError(t, err)

	users, err := repo.AllUsers(ctx)
	require.NoError(t, err)
	require.Len(t, users, 2)

	assert.Equal(t, 101, users[0].Group)
	assert.Equal(t, 1, users[0].Subgroup)
	assert.Equal(t, 101, users[1].Group)
	assert.Equal(t, 2, users[1].Subgroup)

	notifUsers, err := repo.UsersForEveningNotification(ctx)
	require.NoError(t, err)
	require.Len(t, notifUsers, 2)
	assert.Equal(t, 1, notifUsers[0].Subgroup)
	assert.Equal(t, 2, notifUsers[1].Subgroup)
}

func TestUserRepository_AllUsersWithUsernames_And_CountUsers(t *testing.T) {
	db, repo := setupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	err := repo.SaveUser(ctx, entities.User{ChatID: 2001, Username: "alice"})
	require.NoError(t, err)
	err = repo.SetUserGroup(ctx, 2001, 88, 1)
	require.NoError(t, err)

	err = repo.SaveUser(ctx, entities.User{ChatID: 2002, Username: ""})
	require.NoError(t, err)
	err = repo.SetUserGroup(ctx, 2002, 88, 0)
	require.NoError(t, err)

	err = repo.SaveUser(ctx, entities.User{ChatID: 2003, Username: "teacher_bob"})
	require.NoError(t, err)
	err = repo.SetTeacher(ctx, 2003, "Иванов И. И.")
	require.NoError(t, err)

	count, err := repo.CountUsers(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, count)

	users, err := repo.AllUsersWithUsernames(ctx)
	require.NoError(t, err)
	require.Len(t, users, 3)

	var foundAlice, foundTeacher bool
	for _, u := range users {
		if u.ChatID == 2001 {
			foundAlice = true
			assert.Equal(t, "alice", u.Username)
			assert.Equal(t, 88, u.Group)
			assert.Equal(t, 1, u.Subgroup)
		}
		if u.ChatID == 2003 {
			foundTeacher = true
			assert.Equal(t, "teacher_bob", u.Username)
			assert.Equal(t, "Иванов И. И.", u.TeacherName)
		}
	}
	assert.True(t, foundAlice)
	assert.True(t, foundTeacher)
}
