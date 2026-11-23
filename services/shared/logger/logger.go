// Package logger menyiapkan structured logging (slog) untuk semua service.
package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Setup memasang slog sebagai logger default dan mengembalikannya.
//
// Format dan level dibaca dari env: LOG_FORMAT=json|text (default json), LOG_LEVEL=debug|info|warn|error
// (default info). Karena slog.SetDefault juga mengalihkan paket `log` bawaan, pemanggilan
// log.Printf yang masih tersisa ikut keluar sebagai log terstruktur (level INFO).
func Setup(service string) *slog.Logger {
	return setup(service, os.Stdout)
}

func setup(service string, w io.Writer) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(os.Getenv("LOG_LEVEL"))}

	var h slog.Handler
	if strings.EqualFold(os.Getenv("LOG_FORMAT"), "text") {
		h = slog.NewTextHandler(w, opts)
	} else {
		h = slog.NewJSONHandler(w, opts)
	}

	l := slog.New(h).With("service", service)
	slog.SetDefault(l)
	return l
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Fatalf mencatat pesan di level ERROR lalu keluar dengan status 1 (pengganti log.Fatalf).
func Fatalf(format string, args ...any) {
	slog.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}
