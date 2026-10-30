//go:debug tlsmlkem=0
package main

import (
	"flag"
	"log"
	"log/slog"
	"os"

	"github.com/arseniizyk/mgkct-schedule-bot/internal/app"
	"github.com/arseniizyk/mgkct-schedule-bot/internal/config"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config_path", "", "path to config file")
	flag.Parse()

	cfg, err := config.New(configPath)
	if err != nil {
		log.Fatalf("can't initialize config: %s", err)
	}

	logger := setupLogger(cfg.Env)

	application, err := app.New(logger, cfg)
	if err != nil {
		logger.Error("failed to initialize application", "error", err)
		os.Exit(1)
	}

	if err := application.Run(); err != nil {
		logger.Error("application exited with error", "error", err)
		os.Exit(1)
	}
}

func setupLogger(env config.Env) *slog.Logger {
	var handler slog.Handler

	switch env {
	case config.EnvDev:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})
	case config.EnvProd:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	default:
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})
	}

	return slog.New(handler)
}
