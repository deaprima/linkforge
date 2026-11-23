package logger

import (
	"bytes"
	"encoding/json"
	"log"
	"log/slog"
	"strings"
	"testing"
)

func TestSetup_JSONWithServiceAttr(t *testing.T) {
	t.Setenv("LOG_FORMAT", "")
	t.Setenv("LOG_LEVEL", "")
	var buf bytes.Buffer
	l := setup("gateway", &buf)

	l.Info("hello", "k", "v")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("not json: %q: %v", buf.String(), err)
	}
	if rec["service"] != "gateway" || rec["msg"] != "hello" || rec["k"] != "v" || rec["level"] != "INFO" {
		t.Errorf("record = %v", rec)
	}
}

func TestSetup_TextFormat(t *testing.T) {
	t.Setenv("LOG_FORMAT", "text")
	var buf bytes.Buffer
	setup("auth", &buf).Info("hello")
	if !strings.Contains(buf.String(), "msg=hello") || !strings.Contains(buf.String(), "service=auth") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestSetup_LevelFilter(t *testing.T) {
	t.Setenv("LOG_LEVEL", "warn")
	var buf bytes.Buffer
	l := setup("link", &buf)
	l.Info("hidden")
	l.Warn("shown")
	if strings.Contains(buf.String(), "hidden") || !strings.Contains(buf.String(), "shown") {
		t.Errorf("output = %q", buf.String())
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"": slog.LevelInfo, "debug": slog.LevelDebug, "DEBUG": slog.LevelDebug, " warn ": slog.LevelWarn,
		"warning": slog.LevelWarn, "error": slog.LevelError, "bogus": slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

// Pemanggilan log.Printf lama harus ikut menjadi log terstruktur.
func TestSetup_BridgesStdlibLog(t *testing.T) {
	t.Setenv("LOG_FORMAT", "")
	var buf bytes.Buffer
	setup("analytics", &buf)
	log.Printf("legacy %d", 42)

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("not json: %q: %v", buf.String(), err)
	}
	if rec["msg"] != "legacy 42" || rec["service"] != "analytics" {
		t.Errorf("record = %v", rec)
	}
}
