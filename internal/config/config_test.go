package config_test

import (
	"testing"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_AdminIDs(t *testing.T) {
	t.Run("multiple admin IDs", func(t *testing.T) {
		t.Setenv("BOT_TOKEN", "test_token")
		t.Setenv("BOT_ADMIN_IDS", "999,888,777")

		cfg, err := config.New("")
		require.NoError(t, err)
		assert.Equal(t, []int64{999, 888, 777}, cfg.Bot.AdminIDs)
	})

	t.Run("single admin ID", func(t *testing.T) {
		t.Setenv("BOT_TOKEN", "test_token")
		t.Setenv("BOT_ADMIN_IDS", "123456789")

		cfg, err := config.New("")
		require.NoError(t, err)
		assert.Equal(t, []int64{123456789}, cfg.Bot.AdminIDs)
	})

	t.Run("empty admin IDs", func(t *testing.T) {
		t.Setenv("BOT_TOKEN", "test_token")
		t.Setenv("BOT_ADMIN_IDS", "")

		cfg, err := config.New("")
		require.NoError(t, err)
		assert.Empty(t, cfg.Bot.AdminIDs)
	})
}
