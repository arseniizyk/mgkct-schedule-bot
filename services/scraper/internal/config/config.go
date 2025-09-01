package config

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port string
}

func New() (*Config, error) {
	if err := godotenv.Load("../.env"); err != nil {
		slog.Error("can't load .env file, put in in the root of service", "err", err)
		return nil, err
	}

	return &Config{
		Port: os.Getenv("PORT"),
	}, nil
}
