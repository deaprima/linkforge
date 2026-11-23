package handler_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	authpb "github.com/deaprima/linkforge/services/shared/proto/auth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestForgotPassword(t *testing.T) {
	env := newEnv()
	var got *authpb.RequestPasswordResetRequest
	env.auth.requestPasswordReset = func(_ context.Context, r *authpb.RequestPasswordResetRequest) (*authpb.RequestPasswordResetResponse, error) {
		got = r
		return &authpb.RequestPasswordResetResponse{Success: true}, nil
	}

	w := env.do(t, "POST", "/api/v1/auth/forgot-password", `{"email":"dea@example.com"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	if got == nil || got.Email != "dea@example.com" {
		t.Fatalf("email not forwarded: %v", got)
	}
	data := decodeData[map[string]any](t, w)
	if !strings.Contains(data["message"].(string), "If that email is registered") {
		t.Errorf("unexpected message: %v", data)
	}
	// Header rate limit selalu ada, termasuk di respons sukses
	if w.Header().Get("X-RateLimit-Limit") != "3" || w.Header().Get("X-RateLimit-Remaining") != "2" {
		t.Errorf("rate limit headers = %v", w.Header())
	}
}

func TestForgotPassword_SameResponseForUnknownEmail(t *testing.T) {
	// Auth Service membalas sukses untuk email tak terdaftar; Gateway tidak boleh membedakan
	env := newEnv()
	env.auth.requestPasswordReset = func(context.Context, *authpb.RequestPasswordResetRequest) (*authpb.RequestPasswordResetResponse, error) {
		return &authpb.RequestPasswordResetResponse{Success: true}, nil
	}
	a := env.do(t, "POST", "/api/v1/auth/forgot-password", `{"email":"known@example.com"}`)
	b := env.do(t, "POST", "/api/v1/auth/forgot-password", `{"email":"unknown@example.com"}`)
	if a.Code != b.Code || a.Body.String() != b.Body.String() {
		t.Fatalf("responses differ:\n%s\n%s", a.Body.String(), b.Body.String())
	}
}

func TestForgotPassword_Errors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		rpcErr     error
		wantStatus int
		wantCode   string
	}{
		{"missing email", `{}`, nil, 400, "VALIDATION_ERROR"},
		{"malformed email", `{"email":"nope"}`, nil, 400, "VALIDATION_ERROR"},
		{"auth service down", `{"email":"a@b.com"}`, status.Error(codes.Unavailable, "dial"), 503, "SERVICE_UNAVAILABLE"},
		{"internal error hidden", `{"email":"a@b.com"}`, status.Error(codes.Internal, "smtp secret"), 500, "INTERNAL_ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newEnv()
			env.auth.requestPasswordReset = func(context.Context, *authpb.RequestPasswordResetRequest) (*authpb.RequestPasswordResetResponse, error) {
				return nil, tt.rpcErr
			}
			assertError(t, env.do(t, "POST", "/api/v1/auth/forgot-password", tt.body), tt.wantStatus, tt.wantCode)
		})
	}
}

func TestForgotPassword_RateLimited(t *testing.T) {
	env := newEnv()
	env.auth.requestPasswordReset = func(context.Context, *authpb.RequestPasswordResetRequest) (*authpb.RequestPasswordResetResponse, error) {
		return &authpb.RequestPasswordResetResponse{Success: true}, nil
	}
	for i := 0; i < 3; i++ {
		if w := env.do(t, "POST", "/api/v1/auth/forgot-password", `{"email":"a@b.com"}`); w.Code != http.StatusOK {
			t.Fatalf("request %d: status %d", i, w.Code)
		}
	}
	w := env.do(t, "POST", "/api/v1/auth/forgot-password", `{"email":"a@b.com"}`)
	assertError(t, w, 429, "RATE_LIMIT_EXCEEDED")
	if w.Header().Get("Retry-After") == "" || w.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("429 must carry Retry-After and remaining=0: %v", w.Header())
	}
}

func TestResetPassword(t *testing.T) {
	env := newEnv()
	var got *authpb.ResetPasswordRequest
	env.auth.resetPassword = func(_ context.Context, r *authpb.ResetPasswordRequest) (*authpb.ResetPasswordResponse, error) {
		got = r
		return &authpb.ResetPasswordResponse{Success: true}, nil
	}

	w := env.do(t, "POST", "/api/v1/auth/reset-password", `{"token":"abc123","password":"brandnewpass"}`,
		withCookie("access_token", "stale"), withCookie("refresh_token", "stale"))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	if got == nil || got.Token != "abc123" || got.NewPassword != "brandnewpass" {
		t.Fatalf("request not forwarded: %v", got)
	}
	// Sesi lama di browser dibersihkan
	cleared := map[string]bool{}
	for _, c := range w.Result().Cookies() {
		if c.MaxAge < 0 {
			cleared[c.Name] = true
		}
	}
	if !cleared["access_token"] || !cleared["refresh_token"] {
		t.Errorf("session cookies not cleared: %v", w.Result().Cookies())
	}
}

func TestResetPassword_Errors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		rpcErr     error
		wantStatus int
		wantCode   string
	}{
		{"missing token", `{"password":"brandnewpass"}`, nil, 400, "VALIDATION_ERROR"},
		{"short password", `{"token":"t","password":"short"}`, nil, 400, "VALIDATION_ERROR"},
		{"password over bcrypt limit", `{"token":"t","password":"` + strings.Repeat("x", 73) + `"}`, nil, 400, "VALIDATION_ERROR"},
		{"bad token", `{"token":"t","password":"brandnewpass"}`, status.Error(codes.InvalidArgument, "This reset link is invalid or has expired."), 400, "INVALID_RESET_TOKEN"},
		{"auth service down", `{"token":"t","password":"brandnewpass"}`, status.Error(codes.Unavailable, "dial"), 503, "SERVICE_UNAVAILABLE"},
		{"internal error hidden", `{"token":"t","password":"brandnewpass"}`, status.Error(codes.Internal, "pq: secret"), 500, "INTERNAL_ERROR"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newEnv()
			env.auth.resetPassword = func(context.Context, *authpb.ResetPasswordRequest) (*authpb.ResetPasswordResponse, error) {
				return nil, tt.rpcErr
			}
			assertError(t, env.do(t, "POST", "/api/v1/auth/reset-password", tt.body), tt.wantStatus, tt.wantCode)
		})
	}
}

func TestResetPassword_RateLimited(t *testing.T) {
	env := newEnv()
	env.auth.resetPassword = func(context.Context, *authpb.ResetPasswordRequest) (*authpb.ResetPasswordResponse, error) {
		return nil, status.Error(codes.InvalidArgument, "bad")
	}
	for i := 0; i < 3; i++ {
		env.do(t, "POST", "/api/v1/auth/reset-password", `{"token":"t","password":"brandnewpass"}`)
	}
	assertError(t, env.do(t, "POST", "/api/v1/auth/reset-password", `{"token":"t","password":"brandnewpass"}`), 429, "RATE_LIMIT_EXCEEDED")
}
