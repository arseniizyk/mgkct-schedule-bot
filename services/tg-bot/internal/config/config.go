package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	TelegramToken string
	GRPCPort      string
}

func New() (*Config, error) {
	godotenv.Load("../.env")

	return &Config{
		GRPCPort:      os.Getenv("GRPC_PORT"),
		TelegramToken: os.Getenv("TELEGRAM_TOKEN"),
	}, nil
}
