package config

import (
	"fmt"

	"github.com/ilyakaznacheev/cleanenv"
)

type Env string

const (
	EnvDev  Env = "dev"
	EnvProd Env = "prod"
)

type Config struct {
	Env Env `env:"ENV" env-default:"dev"`

	Scraper ScraperConfig
	Bot     BotConfig
	Nats    NatsConfig
}

type ScraperConfig struct {
	GRPCPort string `env:"SCRAPER_GRPC_PORT" env-default:"9001"`

	DB PostgresConfig `env-prefix:"SCRAPER_DB_"`
}

type BotConfig struct {
	Token      string `env:"TELEGRAM_TOKEN"`
	HealthPort string `env:"BOT_HEALTH_PORT" env-default:"8081"`

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
			return nil, fmt.Errorf("%s: failed to load from env: %w", op, err)
		}
		return &cfg, nil
	}

	if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
		return nil, fmt.Errorf("%s: failed to load from file: %w", op, err)
	}

	return &cfg, nil
}
