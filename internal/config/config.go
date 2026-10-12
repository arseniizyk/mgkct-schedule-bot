package config

import (
	"fmt"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Env string

const (
	EnvDev  Env = "dev"
	EnvProd Env = "prod"
)

type Config struct {
	Env Env `env:"ENV" env-default:"dev"`

	Bot     BotConfig     `env-prefix:"BOT_"`
	DB      DBConfig      `env-prefix:"DB_"`
	Scraper ScraperConfig `env-prefix:"SCRAPER_"`
}

type BotConfig struct {
	Token      string `env:"TOKEN" env-required:"true"`
	HealthPort string `env:"HEALTH_PORT" env-default:"8081"`
}

type DBConfig struct {
	URL      string `env:"URL"`
	Host     string `env:"HOST" env-default:"localhost"`
	Port     string `env:"PORT" env-default:"5432"`
	User     string `env:"USER" env-default:"postgres"`
	Password string `env:"PASSWORD" env-default:"password"`
	Name     string `env:"NAME" env-default:"schedule_bot"`
	SSLMode  string `env:"SSLMODE" env-default:"disable"`
}

func (c *DBConfig) DSN() string {
	if c.URL != "" {
		return c.URL
	}
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.Name,
		c.SSLMode,
	)
}

type ScraperConfig struct {
	Interval      time.Duration `env:"INTERVAL" env-default:"1m"`
	PeakInterval  time.Duration `env:"PEAK_INTERVAL" env-default:"25s"`
	NightInterval time.Duration `env:"NIGHT_INTERVAL" env-default:"20m"`
}

func New(configPath string) (*Config, error) {
	const op = "internal.config.New"
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
