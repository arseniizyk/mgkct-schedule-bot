package server

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/crawler"
)

type HTTPServer struct {
	port string
	sch  *crawler.Schedule
	srv  *http.Server
}

func NewHTTPServer(sch *crawler.Schedule, port string) *HTTPServer {
	return &HTTPServer{
		port: port,
		sch:  sch,
	}
}

func (hs *HTTPServer) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/schedule", func(w http.ResponseWriter, r *http.Request) {
		handleSchedule(hs.sch, w)
	})

	httpSrv := &http.Server{
		Addr:    ":" + hs.port,
		Handler: mux,
	}

	hs.srv = httpSrv

	go func() {
		slog.Info("HTTP server started", "port", hs.port)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("http listen error", "err", err)
		}
	}()
}

func (hs *HTTPServer) Shutdown(ctx context.Context) error {
	if err := hs.srv.Shutdown(ctx); err != nil {
		slog.Error("can't shutdown HTTP Server")
		return err
	}

	return nil
}

func handleSchedule(sch *crawler.Schedule, w http.ResponseWriter) {
	b, err := json.MarshalIndent(sch, "", " ")
	if err != nil {
		http.Error(w, "failed to marshal schedule", http.StatusInternalServerError)
		log.Println("marshal error:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}
