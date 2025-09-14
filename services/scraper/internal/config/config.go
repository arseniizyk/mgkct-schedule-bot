package config

import (
	"log/slog"
	"os"

	"github.com/joho/godotenv"
)

func LoadEnv() error {
	if os.Getenv("IS_DOCKER") == "" {
		if err := godotenv.Load("../../.env"); err != nil {
			slog.Error("can't load .env file, put in in the root of service", "err", err)
			return err
		}
	}

	return nil
}
