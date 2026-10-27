package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	pb "github.com/deaprima/linkforge/services/shared/proto/auth"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func init() { gin.SetMode(gin.TestMode) }

// fakeValidator: "valid-token" & "lf_sk_valid" milik user-1, "down" mensimulasikan Auth Service mati.
type fakeValidator struct{}

func (fakeValidator) ValidateToken(_ context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	switch req.GetAccessToken() {
	case "valid-token":
		return &pb.ValidateTokenResponse{IsValid: true, UserId: "user-1"}, nil
	case "down":
		return nil, status.Error(codes.Unavailable, "auth down")
	}
	return &pb.ValidateTokenResponse{IsValid: false}, nil
}

func (fakeValidator) ValidateApiKey(_ context.Context, req *pb.ValidateApiKeyRequest) (*pb.ValidateApiKeyResponse, error) {
	if req.GetApiKey() == "lf_sk_valid" {
		return &pb.ValidateApiKeyResponse{IsValid: true, UserId: "user-1"}, nil
	}
	return &pb.ValidateApiKeyResponse{IsValid: false}, nil
}

func TestRequireAuth(t *testing.T) {
	tests := []struct {
		name        string
		allowAPIKey bool
		setup       func(r *http.Request)
		wantStatus  int
		wantCode    string
	}{
		{"no credentials", true, func(r *http.Request) {}, 401, "UNAUTHORIZED"},
		{"valid bearer", false, func(r *http.Request) { r.Header.Set("Authorization", "Bearer valid-token") }, 200, ""},
		{"invalid bearer", false, func(r *http.Request) { r.Header.Set("Authorization", "Bearer nope") }, 401, "UNAUTHORIZED"},
		{"non-bearer scheme", false, func(r *http.Request) { r.Header.Set("Authorization", "Basic dXNlcjpwYXNz") }, 401, "UNAUTHORIZED"},
		{"empty bearer", false, func(r *http.Request) { r.Header.Set("Authorization", "Bearer ") }, 401, "UNAUTHORIZED"},
		{"access_token cookie", false, func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: AccessTokenCookie, Value: "valid-token"})
		}, 200, ""},
		{"api key allowed", true, func(r *http.Request) { r.Header.Set("X-API-Key", "lf_sk_valid") }, 200, ""},
		{"api key not allowed on account endpoints", false, func(r *http.Request) { r.Header.Set("X-API-Key", "lf_sk_valid") }, 401, "UNAUTHORIZED"},
		{"invalid api key", true, func(r *http.Request) { r.Header.Set("X-API-Key", "lf_sk_revoked") }, 401, "UNAUTHORIZED"},
		{"auth service unavailable", false, func(r *http.Request) { r.Header.Set("Authorization", "Bearer down") }, 503, "SERVICE_UNAVAILABLE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := gin.New()
			r.GET("/protected", RequireAuth(fakeValidator{}, tt.allowAPIKey), func(c *gin.Context) {
				c.String(http.StatusOK, UserID(c))
			})

			req := httptest.NewRequest(http.MethodGet, "/protected", nil)
			tt.setup(req)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", w.Code, tt.wantStatus, w.Body.String())
			}
			if tt.wantStatus == 200 {
				if w.Body.String() != "user-1" {
					t.Errorf("user_id in context = %q, want user-1", w.Body.String())
				}
				return
			}
			var body struct {
				Error struct{ Code string } `json:"error"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &body)
			if body.Error.Code != tt.wantCode {
				t.Errorf("error code = %q, want %q", body.Error.Code, tt.wantCode)
			}
		})
	}
}
