package main

import (
	"flag"
	"log"
	"log/slog"
	"os"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/app"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config_path", "", "path to config")

	cfg, err := config.New(configPath)
	if err != nil {
		log.Fatalf("can't initialize config: %s", err)
	}

	log := setupLogger(cfg.Env)

	app, err := app.New(log, cfg)
	if err != nil {
		panic(err)
	}

	if err := app.Run(); err != nil {
		panic(err)
	}
}

func setupLogger(env config.Env) *slog.Logger {
	var log *slog.Logger

	switch env {
	case config.EnvDev:
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

	case config.EnvProd:
		log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
	default:
		panic("invalid env property")
	}

	return log
}
