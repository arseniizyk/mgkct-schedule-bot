package main

import (
	"log"
	"log/slog"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/app"
)

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	app, err := app.New()
	if err != nil {
		log.Fatal(err)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
