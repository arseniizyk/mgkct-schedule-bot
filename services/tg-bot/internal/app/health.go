package app

import (
	"context"
	"net"
	"net/http"
	"time"
)

const dbPingTimeout = 2 * time.Second

type Pinger interface {
	Ping(context.Context) error
}

func newHealthServer(db Pinger, port string) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), dbPingTimeout)
		defer cancel()

		if err := db.Ping(ctx); err != nil {
			http.Error(w, "database ping failed", http.StatusServiceUnavailable)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	return &http.Server{
		Addr:              net.JoinHostPort("", port),
		Handler:           mux,
		ReadHeaderTimeout: 3 * time.Second,
	}
}
