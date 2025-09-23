package config

import (
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	BotToken string `envconfig:"TELEGRAM_TOKEN"`

	ScraperURL string `default:"scraper:9001" envconfig:"SCRAPER_URL"`
	NatsURL    string `default:"nats://nats:4222" envconfig:"NATS_URL"`

	PostgresPassword string `default:"password" envconfig:"POSTGRES_PASSWORD"`
	PostgresUser     string `default:"postgres" envconfig:"POSTGRES_USER"`
	PostgresDB       string `default:"postgres" envconfig:"POSTGRES_DB"`
	PostgresSSL      string `default:"disable" envconfig:"POSTGRES_SSL"`
	PostgresPort     string `default:"5432" envconfig:"POSTGRES_PORT"`
}

func New() (*Config, error) {
	var cfg Config

	_ = godotenv.Load("../../.env")
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
