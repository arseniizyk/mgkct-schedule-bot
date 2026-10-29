package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
	"github.com/joho/godotenv"
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
	Token      string  `env:"TOKEN"`
	HealthPort string  `env:"HEALTH_PORT" env-default:"8081"`
	WebAppURL  string  `env:"WEB_APP_URL" env-default:""`
	APIURL     string  `env:"API_URL" env-default:""`
	AdminIDs   []int64 `env:"ADMIN_IDS" env-separator:","`
}

type DBConfig struct {
	Path string `env:"PATH" env-default:"bot.db"`
}

type ScraperConfig struct {
	Interval      time.Duration `env:"INTERVAL" env-default:"1m"`
	PeakInterval  time.Duration `env:"PEAK_INTERVAL" env-default:"25s"`
	NightInterval time.Duration `env:"NIGHT_INTERVAL" env-default:"20m"`
}

func New(configPath string) (*Config, error) {
	const op = "internal.config.New"

	// Автоматическая загрузка configs/.env, если файл существует
	if _, err := os.Stat("configs/.env"); err == nil {
		_ = godotenv.Load("configs/.env")
	}

	var cfg Config

	if configPath != "" {
		if err := cleanenv.ReadConfig(configPath, &cfg); err != nil {
			return nil, fmt.Errorf("%s: failed to load from file %s: %w", op, configPath, err)
		}
	} else {
		if err := cleanenv.ReadEnv(&cfg); err != nil {
			return nil, fmt.Errorf("%s: failed to load from env: %w", op, err)
		}
	}

	normalizeConfig(&cfg)

	if cfg.Bot.Token == "" {
		return nil, fmt.Errorf("%s: bot token is required (set TELEGRAM_TOKEN or BOT_TOKEN)", op)
	}

	return &cfg, nil
}

func normalizeConfig(cfg *Config) {
	// Поддержка legacy TELEGRAM_TOKEN переменной
	if cfg.Bot.Token == "" {
		if legacyToken := os.Getenv("TELEGRAM_TOKEN"); legacyToken != "" {
			cfg.Bot.Token = legacyToken
		}
	}
}
