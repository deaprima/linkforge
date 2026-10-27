package router

import (
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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

func TestLogFormatter_NeverLogsToken(t *testing.T) {
	line := logFormatter(gin.LogFormatterParams{
		TimeStamp:  time.Now(),
		StatusCode: 101,
		Method:     "GET",
		Path:       "/api/v1/analytics/realtime?token=eyJsecret",
	})
	if strings.Contains(line, "eyJsecret") || !strings.Contains(line, "token=REDACTED") {
		t.Errorf("log line leaks token: %s", line)
	}
}
