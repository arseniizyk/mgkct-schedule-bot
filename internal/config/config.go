package config

import (
	"fmt"
	"os"

	"github.com/ilyakaznacheev/cleanenv"
)

func New(configPath string) (*Config, error) {
	const op = "internal.config.New"

	var cfg Config

	if configPath != "" {
		if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
			return nil, fmt.Errorf("%s: failed to load from file %s: %w", op, configPath, err)
		}
	} else if _, err := os.Stat("configs/.env"); err == nil {
		if err := cleanenv.ReadConfig("configs/.env", &cfg); err != nil {
			return nil, fmt.Errorf("%s: failed to load from configs/.env: %w", op, err)
		}
	} else {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return nil, fmt.Errorf("%s: failed to load from env: %w", op, err)
		}
	}

	if cfg.Bot.Token == "" {
		return nil, fmt.Errorf("%s: bot token is required (set BOT_TOKEN)", op)
	}

	return &cfg, nil
}
