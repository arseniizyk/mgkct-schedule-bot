package main

import (
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/crawler"
)

func main() {
	// cfg, err := config.New()
	// if err != nil {
	// 	log.Fatal(err)
	// }

	crawler := crawler.New()
	crawler.Crawl()
}
