package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/app"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config_path", "", "path to config")

	cfg, err := config.New(configPath)
	if err != nil {
		log.Fatal(fmt.Errorf("config: %w", err))
	}
	slog.SetLogLoggerLevel(slog.LevelDebug)

	app, err := app.New(cfg)
	if err != nil {
		log.Fatal(err)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
