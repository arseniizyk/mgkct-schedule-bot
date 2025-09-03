package config

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HttpPort string
	GRPCPort string
}

func New() (*Config, error) {
	if err := godotenv.Load("../.env"); err != nil {
		slog.Error("can't load .env file, put in in the root of service", "err", err)
		return nil, err
	}

	return &Config{
		HttpPort: os.Getenv("HTTP_PORT"),
		GRPCPort: os.Getenv("GRPC_PORT"),
	}, nil
}
