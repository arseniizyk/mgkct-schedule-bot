package server

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"

	"github.com/arseniizyk/mgkct-schedule-bot/services/scraper/internal/models"
	scheduleUC "github.com/arseniizyk/mgkct-schedule-bot/services/scraper/pkg/schedule/usecase"
)

type HTTPServer struct {
	port       string
	srv        *http.Server
	scheduleUC *scheduleUC.ScheduleUsecase
}

func NewHTTPServer(schUC *scheduleUC.ScheduleUsecase, port string) *HTTPServer {
	return &HTTPServer{
		port:       port,
		scheduleUC: schUC,
	}
}

func (hs *HTTPServer) Start() {
	mux := http.NewServeMux()
	mux.HandleFunc("/schedule", func(w http.ResponseWriter, r *http.Request) {
		sch, err := hs.scheduleUC.GetLatest()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		handleSchedule(sch, w)
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

func handleSchedule(sch *models.Schedule, w http.ResponseWriter) {
	b, err := json.MarshalIndent(sch, "", " ")
	if err != nil {
		http.Error(w, "failed to marshal schedule", http.StatusInternalServerError)
		log.Println("marshal error:", err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(b)
}
