package utils

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

func HashJSON[T any](t T) ([32]byte, error) {
	var zero [32]byte
	b, err := json.Marshal(t)
	if err != nil {
		slog.Error("hash: can't marshal", "JSON", t, "err", err)
		return zero, fmt.Errorf("hash: marshal: %w", err)
	}
	return sha256.Sum256(b), nil
}

func CleanText(s string) string {
	s = strings.ReplaceAll(s, "\u00a0", "")
	return strings.TrimSpace(s)
}
