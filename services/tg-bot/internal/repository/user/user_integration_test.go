//go:build integration

package user

import (
	"context"
	"errors"
	"testing"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/entities"
	domainerr "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/domain/errors"
	infradb "github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/infrastructure/db"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

func newTestRepo(t *testing.T) (*UserRepository, func()) {
	t.Helper()

	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("users"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("password"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)

	host, err := pgContainer.Host(ctx)
	require.NoError(t, err)

	port, err := pgContainer.MappedPort(ctx, "5432")
	require.NoError(t, err)

	cfg := &config.PostgresConfig{
		Host:     host,
		Port:     port.Port(),
		User:     "postgres",
		Password: "password",
		DBName:   "users",
	}

	pool, err := infradb.Connect(ctx, cfg)
	require.NoError(t, err, "db.Connect должен применить goose-миграции")

	return New(pool), func() {
		pool.Close()
		_ = pgContainer.Terminate(ctx)
	}
}

func TestUserRepositoryIntegration(t *testing.T) {
	repo, cleanup := newTestRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("save and get all users", func(t *testing.T) {
		require.NoError(t, repo.SaveUser(ctx, entities.NewUser(100, "alice")))
		require.NoError(t, repo.SaveUser(ctx, entities.NewUser(101, "bob")))

		users, err := repo.AllUsers(ctx)
		require.NoError(t, err)
		require.Len(t, users, 2)
	})

	t.Run("save upserts username on conflict", func(t *testing.T) {
		require.NoError(t, repo.SaveUser(ctx, entities.NewUser(100, "alice2")))

		users, err := repo.AllUsers(ctx)
		require.NoError(t, err)
		require.Len(t, users, 2, "chat_id должен остаться один за счёт ON CONFLICT")
	})

	t.Run("group by chat id without group", func(t *testing.T) {
		_, err := repo.GroupByChatID(ctx, 100)
		require.True(t, errors.Is(err, domainerr.ErrUserNoGroup))
	})

	t.Run("group by unknown chat id", func(t *testing.T) {
		_, err := repo.GroupByChatID(ctx, 999)
		require.True(t, errors.Is(err, domainerr.ErrUserNoGroup))
	})

	t.Run("set user group then get it back", func(t *testing.T) {
		require.NoError(t, repo.SetUserGroup(ctx, 100, 99))

		group, err := repo.GroupByChatID(ctx, 100)
		require.NoError(t, err)
		require.Equal(t, 99, group)
	})

	t.Run("set user group updates existing", func(t *testing.T) {
		require.NoError(t, repo.SetUserGroup(ctx, 100, 55))

		group, err := repo.GroupByChatID(ctx, 100)
		require.NoError(t, err)
		require.Equal(t, 55, group)
	})

	t.Run("user ids by group id", func(t *testing.T) {
		ids, err := repo.UserIDsByGroupID(ctx, 55)
		require.NoError(t, err)
		require.Equal(t, []int64{100}, ids)
	})

	t.Run("user ids by empty group", func(t *testing.T) {
		ids, err := repo.UserIDsByGroupID(ctx, 12345)
		require.NoError(t, err)
		require.Empty(t, ids)
	})
}
