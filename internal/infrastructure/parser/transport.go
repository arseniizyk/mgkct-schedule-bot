package parser

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"github.com/gocolly/colly"
)

const (
	requestTimeout = 12 * time.Second
	maxAttempts    = 3
	retryDelay     = 1 * time.Second
	defaultUA      = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
)

var (
	sharedTransport     *http.Transport
	sharedTransportOnce sync.Once
)

// newCollector создает преднастроенный экземпляр colly.Collector с общим HTTP-транспортом и таймаутом.
func newCollector() *colly.Collector {
	c := colly.NewCollector(
		colly.AllowURLRevisit(),
		colly.UserAgent(defaultUA),
	)
	c.WithTransport(getSharedTransport())
	c.SetRequestTimeout(requestTimeout)
	return c
}

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
