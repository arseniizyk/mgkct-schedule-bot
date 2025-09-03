package main

import (
	"log"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/app"
)

func main() {
	app, err := app.New()
	if err != nil {
		log.Fatal(err)
	}

	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
