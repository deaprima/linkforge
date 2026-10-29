// Package ops menjalankan listener HTTP internal (/metrics dan /healthz) untuk sebuah service.
//
// Listener ini sengaja terpisah dari port publik: /metrics tidak boleh terjangkau dari internet
// (lihat API-CONTRACT.md bagian Ops & Observability). Batasi aksesnya ke jaringan Prometheus
// lewat firewall / bind address.
package ops

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler berisi /metrics dan /healthz.
func Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	return mux
}

// Serve menjalankan listener ops di background dan mengembalikan fungsi shutdown.
// addr kosong = dinonaktifkan.
func Serve(addr string) (shutdown func(context.Context) error) {
	if addr == "" {
		slog.Info("ops listener disabled (metrics address is empty)")
		return func(context.Context) error { return nil }
	}
	srv := &http.Server{Addr: addr, Handler: Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		slog.Info("ops listener running", "addr", addr, "endpoints", "/metrics /healthz")
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("ops listener failed", "addr", addr, "error", err)
		}
	}()
	return srv.Shutdown
}

// AddrFromEnv membaca alamat listener ops dari env. Kalau variabel tidak di-set dipakai def;
// kalau di-set kosong, listener dinonaktifkan.
func AddrFromEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(v)
	}
	return def
}
