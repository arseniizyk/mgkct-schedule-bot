package widget

import (
	"bytes"
	"compress/zlib"
	"fmt"
)

const (
	// MaxUncompressedSize is the Telegram Widget Profile 1 limit (32 KB).
	MaxUncompressedSize = 32768
	// MaxCompressedSize is the Telegram zlib payload limit (4 KB).
	MaxCompressedSize = 4096
)

// Compress compresses the HTML using zlib with BestCompression (level 9).
func Compress(html string) ([]byte, error) {
	var buf bytes.Buffer
	w, err := zlib.NewWriterLevel(&buf, zlib.BestCompression)
	if err != nil {
		return nil, fmt.Errorf("widget: create zlib writer: %w", err)
	}

	if _, err := w.Write([]byte(html)); err != nil {
		_ = w.Close()
		return nil, fmt.Errorf("widget: compress payload: %w", err)
	}

	if err := w.Close(); err != nil {
		return nil, fmt.Errorf("widget: close zlib writer: %w", err)
	}

	return buf.Bytes(), nil
}

// ValidateSize verifies that uncompressed and compressed sizes satisfy Telegram limits.
func ValidateSize(html string) (rawLen, compLen int, err error) {
	rawLen = len(html)
	if rawLen > MaxUncompressedSize {
		return rawLen, 0, fmt.Errorf("widget HTML exceeds uncompressed limit of %d bytes (got %d)", MaxUncompressedSize, rawLen)
	}

	compressed, err := Compress(html)
	if err != nil {
		return rawLen, 0, err
	}
	compLen = len(compressed)

	if compLen > MaxCompressedSize {
		return rawLen, compLen, fmt.Errorf("widget compressed size exceeds limit of %d bytes (got %d)", MaxCompressedSize, compLen)
	}

	return rawLen, compLen, nil
}
