package main

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/config"
	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/crawler"
)

func main() {
	cfg, err := config.New()
	if err != nil {
		log.Fatal(err)
	}

	c := crawler.New()

	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		schedule, err := c.Crawl()
		if err != nil {
			http.Error(w, "failed to crawl", http.StatusInternalServerError)
			log.Println("crawl error:", err)
			return
		}

		b, err := json.MarshalIndent(schedule, "", " ")
		if err != nil {
			http.Error(w, "failed to marshal schedule", http.StatusInternalServerError)
			log.Println("marshal error:", err)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(b)
	})

	log.Println("server started on", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, nil))
}
