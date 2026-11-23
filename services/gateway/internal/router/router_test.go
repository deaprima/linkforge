package router

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestRedactQuery(t *testing.T) {
	tests := []struct {
		path, want string
	}{
		{"/api/v1/links", "/api/v1/links"},
		{"/api/v1/links?page=2", "/api/v1/links?page=2"},
		{"/api/v1/analytics/realtime?token=eyJsecret", "/api/v1/analytics/realtime?token=REDACTED"},
		{"/api/v1/auth/oauth/google/callback?code=4/0Asecret&state=abc", "/api/v1/auth/oauth/google/callback?code=REDACTED&state=REDACTED"},
		{"/x?page=1&token=eyJsecret", "/x?page=1&token=REDACTED"},
		{"/x?%zz", "/x?REDACTED"}, // query tidak bisa di-parse → redact semuanya
	}
	for _, tt := range tests {
		if got := redactQuery(tt.path); got != tt.want {
			t.Errorf("redactQuery(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

// captureLogs mengalihkan slog default ke buffer selama test berjalan.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func observeEngine() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(observe(), gin.Recovery())
	r.GET("/items/:id", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	r.GET("/boom", func(c *gin.Context) { panic("boom") })
	return r
}

func lastLog(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &rec); err != nil {
		t.Fatalf("log is not json: %q: %v", buf.String(), err)
	}
	return rec
}

func TestObserve_NeverLogsToken(t *testing.T) {
	buf := captureLogs(t)
	w := httptest.NewRecorder()
	observeEngine().ServeHTTP(w, httptest.NewRequest("GET", "/items/1?token=eyJsecret", nil))

	if strings.Contains(buf.String(), "eyJsecret") || !strings.Contains(buf.String(), "token=REDACTED") {
		t.Errorf("access log leaks token: %s", buf.String())
	}
}

func TestObserve_AccessLogFields(t *testing.T) {
	buf := captureLogs(t)
	w := httptest.NewRecorder()
	observeEngine().ServeHTTP(w, httptest.NewRequest("GET", "/items/42", nil))

	rec := lastLog(t, buf)
	if rec["msg"] != "http request" || rec["method"] != "GET" || rec["route"] != "/items/:id" || rec["status"] != float64(200) {
		t.Errorf("record = %v", rec)
	}
	if rec["request_id"] == "" || rec["request_id"] != w.Header().Get("X-Request-ID") {
		t.Errorf("request_id in log %v must equal response header %q", rec["request_id"], w.Header().Get("X-Request-ID"))
	}
}

func TestObserve_RequestID(t *testing.T) {
	cases := []struct {
		name, in string
		keep     bool
	}{
		{"valid id is kept", "abc-123_XYZ", true},
		{"empty gets generated", "", false},
		{"too long gets replaced", strings.Repeat("a", 65), false},
		{"unsafe characters get replaced", "bad id\nforged=1", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			captureLogs(t)
			req := httptest.NewRequest("GET", "/items/1", nil)
			if tc.in != "" {
				req.Header.Set("X-Request-ID", tc.in)
			}
			w := httptest.NewRecorder()
			observeEngine().ServeHTTP(w, req)

			got := w.Header().Get("X-Request-ID")
			if got == "" || (tc.keep && got != tc.in) || (!tc.keep && got == tc.in) {
				t.Errorf("X-Request-ID = %q (in %q, keep=%v)", got, tc.in, tc.keep)
			}
		})
	}
}

func TestObserve_MetricsUseRouteTemplate(t *testing.T) {
	captureLogs(t)
	r := observeEngine()
	before := testutil.ToFloat64(httpRequests.WithLabelValues("GET", "/items/:id", "200"))
	beforeNF := testutil.ToFloat64(httpRequests.WithLabelValues("GET", "unmatched", "404"))

	for _, p := range []string{"/items/1", "/items/2", "/items/3", "/nope/random-1", "/nope/random-2"} {
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", p, nil))
	}

	// Banyak path berbeda → satu series; path tak dikenal tidak membuat label baru per URL
	if got := testutil.ToFloat64(httpRequests.WithLabelValues("GET", "/items/:id", "200")) - before; got != 3 {
		t.Errorf("route template count = %v, want 3", got)
	}
	if got := testutil.ToFloat64(httpRequests.WithLabelValues("GET", "unmatched", "404")) - beforeNF; got != 2 {
		t.Errorf("unmatched count = %v, want 2", got)
	}
}

func TestObserve_PanicIsLoggedAsError(t *testing.T) {
	buf := captureLogs(t)
	w := httptest.NewRecorder()
	observeEngine().ServeHTTP(w, httptest.NewRequest("GET", "/boom", nil))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.Code)
	}
	rec := lastLog(t, buf)
	if rec["level"] != "ERROR" || rec["status"] != float64(500) {
		t.Errorf("record = %v", rec)
	}
}
