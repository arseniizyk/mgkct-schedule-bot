package main

import (
	"log"

	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/app"
	"github.com/arseniizyk/mgkct-schedule-bot/services/tg-bot/internal/config"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatal(err)
	}

	a := app.New(cfg)
	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}
