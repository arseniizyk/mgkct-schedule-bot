package main

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/arseniizyk/mgkct-schedule-bot/libs/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/app"
)

func main() {
	cfg, err := config.New()
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
