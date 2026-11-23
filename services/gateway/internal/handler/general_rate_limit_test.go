package handler_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	linkpb "github.com/deaprima/linkforge/services/shared/proto/link"
)

func TestAuthEndpoints_RateLimitedPerIP(t *testing.T) {
	env := newEnv()
	env.authLimit = 3

	for i := 1; i <= 3; i++ {
		w := env.do(t, "POST", "/api/v1/auth/login", `{}`, fromIP("198.51.100.9"))
		if w.Code == http.StatusTooManyRequests {
			t.Fatalf("attempt %d blocked too early", i)
		}
		if rem := w.Header().Get("X-RateLimit-Remaining"); rem != strconv.Itoa(3-i) {
			t.Errorf("attempt %d: X-RateLimit-Remaining = %q, want %d", i, rem, 3-i)
		}
	}
	w := env.do(t, "POST", "/api/v1/auth/login", `{}`, fromIP("198.51.100.9"))
	assertError(t, w, 429, "RATE_LIMIT_EXCEEDED")
	if w.Header().Get("Retry-After") == "" {
		t.Error("missing Retry-After on 429")
	}

	// Satu bucket untuk register/login/refresh/OAuth dari IP yang sama
	assertError(t, env.do(t, "POST", "/api/v1/auth/register", `{}`, fromIP("198.51.100.9")), 429, "RATE_LIMIT_EXCEEDED")
	// IP lain tidak terpengaruh
	if w := env.do(t, "POST", "/api/v1/auth/login", `{}`, fromIP("203.0.113.50")); w.Code == http.StatusTooManyRequests {
		t.Error("other IP must not share the quota")
	}
}

func TestAuthEndpoints_FailClosedWhenRedisDown(t *testing.T) {
	env := newEnv()
	env.redis.Close()
	assertError(t, env.do(t, "POST", "/api/v1/auth/login", `{"email":"a@b.com","password":"x"}`), 503, "SERVICE_UNAVAILABLE")
}

func TestRedirect_RateLimitedPerIP(t *testing.T) {
	env := newEnv()
	env.publicLimit = 2
	env.link.resolveAlias = func(context.Context, *linkpb.ResolveAliasRequest) (*linkpb.ResolveAliasResponse, error) {
		return &linkpb.ResolveAliasResponse{OriginalUrl: "https://example.com"}, nil
	}

	for i := 0; i < 2; i++ {
		if w := env.do(t, "GET", "/open-link", "", fromIP("198.51.100.9")); w.Code != http.StatusFound {
			t.Fatalf("request %d: status %d", i+1, w.Code)
		}
	}
	assertError(t, env.do(t, "GET", "/open-link", "", fromIP("198.51.100.9")), 429, "RATE_LIMIT_EXCEEDED")
	if w := env.do(t, "GET", "/open-link", "", fromIP("203.0.113.50")); w.Code != http.StatusFound {
		t.Errorf("other IP blocked: %d", w.Code)
	}
}

func TestRedirect_FailOpenWhenRedisDown(t *testing.T) {
	env := newEnv()
	env.link.resolveAlias = func(context.Context, *linkpb.ResolveAliasRequest) (*linkpb.ResolveAliasResponse, error) {
		return &linkpb.ResolveAliasResponse{OriginalUrl: "https://example.com"}, nil
	}
	env.redis.Close()
	if w := env.do(t, "GET", "/open-link", ""); w.Code != http.StatusFound {
		t.Errorf("redirect must keep working without redis: %d", w.Code)
	}
}

func TestAuthenticatedEndpoints_RateLimitedPerUser(t *testing.T) {
	env := newEnv()
	env.userLimit = 3

	// Bearer dan API key milik user yang sama berbagi satu kuota
	for i, opt := range []func(*http.Request){bearer, apiKey, bearer} {
		if w := env.do(t, "GET", "/api/v1/links", "", opt); w.Code == http.StatusTooManyRequests {
			t.Fatalf("request %d blocked too early", i+1)
		}
	}
	assertError(t, env.do(t, "GET", "/api/v1/links", "", apiKey), 429, "RATE_LIMIT_EXCEEDED")
	assertError(t, env.do(t, "GET", "/api/v1/analytics/summary", "", bearer), 429, "RATE_LIMIT_EXCEEDED")
	assertError(t, env.do(t, "GET", "/api/v1/auth/me", "", bearer), 429, "RATE_LIMIT_EXCEEDED")
}

func TestAuthenticatedEndpoints_InvalidCredentialsDoNotConsumeUserQuota(t *testing.T) {
	env := newEnv()
	env.userLimit = 1

	// Gagal auth → 401 sebelum limiter per-user (tidak ada user ID untuk dihitung)
	for i := 0; i < 3; i++ {
		assertError(t, env.do(t, "GET", "/api/v1/links", ""), 401, "UNAUTHORIZED")
	}
	if w := env.do(t, "GET", "/api/v1/links", "", bearer); w.Code == http.StatusTooManyRequests {
		t.Error("unauthenticated attempts must not consume the user's quota")
	}
}

func TestOpsEndpoints_NotRateLimited(t *testing.T) {
	env := newEnv()
	env.publicLimit, env.authLimit, env.userLimit = 1, 1, 1
	for i := 0; i < 5; i++ {
		if w := env.do(t, "GET", "/healthz", ""); w.Code != http.StatusOK {
			t.Fatalf("healthz request %d: %d", i+1, w.Code)
		}
	}
}

// Landing page memanggil /auth/refresh untuk setiap pengunjung anonim; itu tidak boleh menghabiskan
// kuota login/register milik orang lain di IP yang sama (kantor, kampus, NAT).
func TestRefresh_HasItsOwnBucketSeparateFromLogin(t *testing.T) {
	env := newEnv()
	env.authLimit = 2
	env.refreshLimit = 5

	for i := 0; i < 5; i++ {
		if w := env.do(t, "POST", "/api/v1/auth/refresh", `{}`, fromIP("198.51.100.9")); w.Code == http.StatusTooManyRequests {
			t.Fatalf("refresh %d blocked too early", i+1)
		}
	}
	assertError(t, env.do(t, "POST", "/api/v1/auth/refresh", `{}`, fromIP("198.51.100.9")), 429, "RATE_LIMIT_EXCEEDED")

	// Kuota login di IP yang sama masih utuh
	for i := 0; i < 2; i++ {
		if w := env.do(t, "POST", "/api/v1/auth/login", `{}`, fromIP("198.51.100.9")); w.Code == http.StatusTooManyRequests {
			t.Fatalf("login %d must not be affected by refresh traffic", i+1)
		}
	}
}

func TestRefresh_FailOpenWhenRedisDown(t *testing.T) {
	env := newEnv()
	env.redis.Close()
	if w := env.do(t, "POST", "/api/v1/auth/refresh", `{}`); w.Code == http.StatusServiceUnavailable {
		t.Errorf("refresh must not depend on the limiter: %d %s", w.Code, w.Body.String())
	}
}

// Tebakan token / API key yang salah ditolak 401 sebelum limiter per user, jadi harus tetap dibatasi per IP.
func TestAuthenticatedEndpoints_BadCredentialsAreLimitedPerIP(t *testing.T) {
	env := newEnv()
	env.publicLimit = 3

	for i := 1; i <= 3; i++ {
		assertError(t, env.do(t, "GET", "/api/v1/links", "", fromIP("198.51.100.9")), 401, "UNAUTHORIZED")
	}
	assertError(t, env.do(t, "GET", "/api/v1/links", "", fromIP("198.51.100.9")), 429, "RATE_LIMIT_EXCEEDED")
	assertError(t, env.do(t, "GET", "/api/v1/analytics/summary", "", fromIP("198.51.100.9")), 429, "RATE_LIMIT_EXCEEDED")
	assertError(t, env.do(t, "GET", "/api/v1/auth/me", "", fromIP("198.51.100.9")), 429, "RATE_LIMIT_EXCEEDED")

	// IP lain tidak terpengaruh
	assertError(t, env.do(t, "GET", "/api/v1/links", "", fromIP("203.0.113.50")), 401, "UNAUTHORIZED")
}
