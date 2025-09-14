package main

import (
	"log"
	"log/slog"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/app"
)

func main() {
	app, err := app.New()
	if err != nil {
		log.Fatal(err)
	}

	slog.SetLogLoggerLevel(slog.LevelDebug)

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
