package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Scraper ScraperConfig
	Bot     BotConfig
	Nats    NatsConfig
}

type ScraperConfig struct {
	URL string `env:"SCRAPER_URL" env-default:"scraper:9001"`

	DB PostgresConfig `env-prefix:"SCRAPER_DB_"`
}

type BotConfig struct {
	Token string `env:"TELEGRAM_TOKEN"`

	DB PostgresConfig `env-prefix:"BOT_DB_"`
}

type NatsConfig struct {
	URL string `env:"NATS_URL" env-default:"nats://nats:4222"`
}

type PostgresConfig struct {
	Host     string `env:"HOST" env-required:"true"`
	Port     string `env:"PORT" env-default:"5432"`
	User     string `env:"USER" env-default:"postgres"`
	Password string `env:"PASSWORD" env-default:"password"`
	DBName   string `env:"NAME" env-required:"true"`
}

func New(configPath string) (*Config, error) {
	const op = "libs.config.New"
	var cfg Config

	if configPath == "" {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return nil, fmt.Errorf("%s: failed to load from env: %s", op, err)
		}
		return &cfg, nil
	}

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("%s: failed to load from file: %s", op, err)
	}

	return &cfg, nil
}
