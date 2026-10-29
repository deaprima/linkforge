package ops

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler(t *testing.T) {
	h := Handler()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/healthz", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Errorf("healthz = %d %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/metrics", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "go_goroutines") {
		t.Errorf("metrics = %d, body lacks go runtime metrics", w.Code)
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/anything-else", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("unknown path = %d, want 404", w.Code)
	}
}

func TestServe_EmptyAddrIsDisabled(t *testing.T) {
	shutdown := Serve("")
	if err := shutdown(t.Context()); err != nil {
		t.Errorf("shutdown of disabled listener: %v", err)
	}
}

func TestAddrFromEnv(t *testing.T) {
	t.Setenv("OPS_TEST_SET", " :9999 ")
	t.Setenv("OPS_TEST_EMPTY", "")

	if got := AddrFromEnv("OPS_TEST_SET", ":1"); got != ":9999" {
		t.Errorf("set = %q", got)
	}
	if got := AddrFromEnv("OPS_TEST_EMPTY", ":1"); got != "" {
		t.Errorf("explicitly empty must disable, got %q", got)
	}
	if got := AddrFromEnv("OPS_TEST_UNSET_XYZ", ":1"); got != ":1" {
		t.Errorf("unset must use default, got %q", got)
	}
}
