package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deaprima/linkforge/services/shared/breaker"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func init() { gin.SetMode(gin.TestMode) }

func newContext() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	return c, w
}

func decodeError(t *testing.T, w *httptest.ResponseRecorder) ErrorBody {
	t.Helper()
	var body errorEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json %q: %v", w.Body.String(), err)
	}
	if body.Success {
		t.Fatal("error envelope must have success=false")
	}
	return body.Error
}

func TestGRPCError_DefaultMappings(t *testing.T) {
	tests := []struct {
		code       codes.Code
		wantStatus int
		wantCode   string
	}{
		{codes.InvalidArgument, 400, CodeValidation},
		{codes.FailedPrecondition, 400, CodeValidation},
		{codes.Unauthenticated, 401, CodeUnauthorized},
		{codes.PermissionDenied, 403, CodeForbidden},
		{codes.NotFound, 404, CodeNotFound},
		{codes.AlreadyExists, 409, CodeConflict},
		{codes.ResourceExhausted, 429, CodeRateLimitExceeded},
		{codes.Unavailable, 503, CodeServiceUnavailable},
		{codes.DeadlineExceeded, 503, CodeServiceUnavailable},
		{codes.Internal, 500, CodeInternal},
		{codes.Unknown, 500, CodeInternal},
	}
	for _, tt := range tests {
		t.Run(tt.code.String(), func(t *testing.T) {
			c, w := newContext()
			GRPCError(c, status.Error(tt.code, "detail from service"), nil)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if got := decodeError(t, w); got.Code != tt.wantCode {
				t.Errorf("code = %s, want %s", got.Code, tt.wantCode)
			}
			if !c.IsAborted() {
				t.Error("handler chain must be aborted")
			}
		})
	}
}

func TestGRPCError_MessageExposure(t *testing.T) {
	t.Run("4xx passes service message through", func(t *testing.T) {
		c, w := newContext()
		GRPCError(c, status.Error(codes.NotFound, "link not found"), nil)
		if msg := decodeError(t, w).Message; msg != "link not found" {
			t.Errorf("message = %q", msg)
		}
	})
	t.Run("5xx hides internal details", func(t *testing.T) {
		c, w := newContext()
		GRPCError(c, status.Error(codes.Internal, "pq: password authentication failed for user linkforge"), nil)
		if msg := decodeError(t, w).Message; strings.Contains(msg, "pq:") || strings.Contains(msg, "linkforge") {
			t.Errorf("internal detail leaked: %q", msg)
		}
	})
	t.Run("503 hides internal details", func(t *testing.T) {
		c, w := newContext()
		GRPCError(c, status.Error(codes.Unavailable, "dial tcp 10.0.0.5:50051: connection refused"), nil)
		if msg := decodeError(t, w).Message; strings.Contains(msg, "10.0.0.5") {
			t.Errorf("internal detail leaked: %q", msg)
		}
	})
}

func TestGRPCError_OverrideWins(t *testing.T) {
	c, w := newContext()
	GRPCError(c, status.Error(codes.AlreadyExists, "email already exists"), Overrides{
		codes.AlreadyExists: {Status: http.StatusConflict, Code: CodeEmailExists},
	})
	if w.Code != 409 || decodeError(t, w).Code != CodeEmailExists {
		t.Errorf("got %d %s", w.Code, w.Body.String())
	}
}

func TestGRPCError_BreakerOpen(t *testing.T) {
	c, w := newContext()
	GRPCError(c, breaker.ErrOpen, nil)
	if w.Code != 503 || decodeError(t, w).Code != CodeServiceUnavailable {
		t.Errorf("got %d %s", w.Code, w.Body.String())
	}
}

func TestGRPCError_NonGRPCError(t *testing.T) {
	c, w := newContext()
	GRPCError(c, errors.New("boom"), nil)
	if w.Code != 500 || decodeError(t, w).Code != CodeInternal {
		t.Errorf("got %d %s", w.Code, w.Body.String())
	}
}

func TestSuccessEnvelopes(t *testing.T) {
	t.Run("OK without meta", func(t *testing.T) {
		c, w := newContext()
		OK(c, http.StatusCreated, gin.H{"id": "1"})
		if w.Code != 201 || w.Body.String() != `{"success":true,"data":{"id":"1"}}` {
			t.Errorf("got %d %s", w.Code, w.Body.String())
		}
	})
	t.Run("OK with meta", func(t *testing.T) {
		c, w := newContext()
		OKWithMeta(c, []int{}, Meta{Page: 2, Limit: 10, Total: 11})
		want := `{"success":true,"data":[],"meta":{"page":2,"limit":10,"total":11}}`
		if w.Body.String() != want {
			t.Errorf("got %s, want %s", w.Body.String(), want)
		}
	})
	t.Run("NoContent", func(t *testing.T) {
		c, w := newContext()
		NoContent(c)
		c.Writer.WriteHeaderNow()
		if w.Code != 204 || w.Body.Len() != 0 {
			t.Errorf("got %d %q", w.Code, w.Body.String())
		}
	})
}
