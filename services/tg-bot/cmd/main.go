package main

import (
	"log"
	"log/slog"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/app"
)

func main() {
	slog.SetLogLoggerLevel(slog.LevelDebug)

	if err := app.New().Run(); err != nil {
		log.Fatal(err)
	}
}
