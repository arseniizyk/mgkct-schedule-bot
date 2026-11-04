package httputil

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// WriteJSON sends a JSON response with status code.
func WriteJSON(w http.ResponseWriter, log *slog.Logger, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil && log != nil {
		log.Error("failed encoding json response", "error", err)
	}
}
