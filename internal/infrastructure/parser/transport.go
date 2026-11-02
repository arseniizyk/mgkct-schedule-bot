package parser

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"
)

const (
	requestTimeout = 12 * time.Second
	maxAttempts    = 3
	retryDelay     = 1 * time.Second
)

var (
	sharedTransport     *http.Transport
	sharedTransportOnce sync.Once
)

// getSharedTransport возвращает синглтон http.Transport с включенным Keep-Alive и HTTP/2,
// переиспользующий открытые соединения между вызовами парсеров.
func getSharedTransport() *http.Transport {
	sharedTransportOnce.Do(func() {
		sharedTransport = &http.Transport{
			DisableKeepAlives:   false,
			ForceAttemptHTTP2:   true,
			MaxIdleConns:        20,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			TLSClientConfig: &tls.Config{
				MinVersion:       tls.VersionTLS12,
				MaxVersion:       tls.VersionTLS13,
				CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
			},
		}
	})
	return sharedTransport
}
