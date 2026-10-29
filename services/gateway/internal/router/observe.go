package router

import (
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const requestIDHeader = "X-Request-ID"

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "HTTP request yang ditangani Gateway, per method, route template, dan status.",
	}, []string{"method", "route", "status"})

	httpLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "Durasi HTTP request di Gateway, per method dan route template.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"method", "route"})
)

// observe memberi setiap request X-Request-ID, mencatat metrik, dan menulis satu access log terstruktur.
// Label route memakai template (/api/v1/links/:id), bukan path asli, supaya kardinalitas metrik terbatas.
func observe() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		id := requestID(c.GetHeader(requestIDHeader))
		c.Set(requestIDHeader, id)
		c.Header(requestIDHeader, id)

		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		latency := time.Since(start)
		code := c.Writer.Status()

		httpRequests.WithLabelValues(c.Request.Method, route, strconv.Itoa(code)).Inc()
		httpLatency.WithLabelValues(c.Request.Method, route).Observe(latency.Seconds())

		level := slog.LevelInfo
		switch {
		case code >= http.StatusInternalServerError:
			level = slog.LevelError
		case route == "/healthz" || route == "/readyz":
			level = slog.LevelDebug // probe berkala, terlalu berisik di level info
		}
		attrs := []slog.Attr{
			slog.String("request_id", id),
			slog.String("method", c.Request.Method),
			slog.String("path", redactQuery(c.Request.URL.RequestURI())),
			slog.String("route", route),
			slog.Int("status", code),
			slog.Float64("latency_ms", float64(latency.Microseconds())/1000),
			slog.String("client_ip", c.ClientIP()),
		}
		if len(c.Errors) > 0 {
			attrs = append(attrs, slog.String("errors", c.Errors.String()))
		}
		slog.LogAttrs(c.Request.Context(), level, "http request", attrs...)
	}
}

// requestID memakai ID dari client/proxy kalau bentuknya aman, kalau tidak membuat yang baru.
// Dibatasi karakter dan panjangnya supaya tidak bisa dipakai menyuntik baris log atau header.
func requestID(incoming string) string {
	if n := len(incoming); n > 0 && n <= 64 && strings.IndexFunc(incoming, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_')
	}) < 0 {
		return incoming
	}
	return uuid.NewString()
}

// sensitiveQueryParams tidak boleh muncul di access log (mis. ?token= di WebSocket realtime).
var sensitiveQueryParams = []string{"token", "code", "state"}

func redactQuery(path string) string {
	base, rawQuery, found := strings.Cut(path, "?")
	if !found {
		return path
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return base + "?REDACTED"
	}
	redacted := false
	for _, key := range sensitiveQueryParams {
		if query.Has(key) {
			query.Set(key, "REDACTED")
			redacted = true
		}
	}
	if !redacted {
		return path
	}
	return base + "?" + query.Encode()
}
