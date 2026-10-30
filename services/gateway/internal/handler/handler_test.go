package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/deaprima/linkforge/services/gateway/internal/handler"
	"github.com/deaprima/linkforge/services/gateway/internal/ratelimit"
	"github.com/deaprima/linkforge/services/gateway/internal/response"
	"github.com/deaprima/linkforge/services/gateway/internal/router"
	"github.com/deaprima/linkforge/services/shared/breaker"
	authpb "github.com/deaprima/linkforge/services/shared/proto/auth"
	linkpb "github.com/deaprima/linkforge/services/shared/proto/link"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	baseURL     = "https://lf.test"
	frontendURL = "https://app.test"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	gin.DefaultWriter = io.Discard
	os.Exit(m.Run())
}

type testEnv struct {
	auth      *fakeAuth
	link      *fakeLink
	analytics *fakeAnalytics
	google    *handler.GoogleOAuth
	checks    map[string]handler.CheckFunc
	redis     *miniredis.Miniredis // state rate limiter, satu per env
	redisAddr string               // disimpan terpisah: Addr() tidak bisa dipanggil setelah Close()

	// Batas rate limit umum per jendela 1 jam; 0 = longgar (1000) supaya test lain tidak ikut terkena limit
	authLimit, refreshLimit, publicLimit, userLimit int
}

func orDefault(n, def int) int {
	if n == 0 {
		return def
	}
	return n
}

func newEnv() *testEnv {
	// Tidak memakai miniredis.RunT: env sering dipakai lintas subtest, sedangkan RunT
	// menutup server saat subtest pertama selesai.
	mr, err := miniredis.Run()
	if err != nil {
		panic(err)
	}
	return &testEnv{
		redis:     mr,
		redisAddr: mr.Addr(),
		auth:      &fakeAuth{},
		link:      &fakeLink{},
		analytics: &fakeAnalytics{},
		google:    handler.NewGoogleOAuth("", "", ""), // OAuth nonaktif secara default
		checks:    map[string]handler.CheckFunc{},
	}
}

func (e *testEnv) engine(t *testing.T) *gin.Engine {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: e.redisAddr})
	t.Cleanup(func() { rdb.Close() })
	// Batas produksi: 5/15 menit per IP+alias, 20/15 menit per IP, fail-closed
	verifyLimit := ratelimit.Middleware(ratelimit.NewRedisLimiter(rdb, "rl:"),
		ratelimit.VerifyPasswordRules(5, 20, 15*time.Minute), ratelimit.Options{Message: "Too many password attempts. Try again later."})

	cookies := handler.CookieConfig{Secure: true, AccessTokenTTL: 15 * time.Minute, RefreshTokenTTL: 7 * 24 * time.Hour}
	engine, err := router.New(router.Handlers{
		Auth:      handler.NewAuthHandler(e.auth, e.google, cookies, frontendURL),
		Link:      handler.NewLinkHandler(e.link, baseURL),
		Redirect:  handler.NewRedirectHandler(e.link, frontendURL),
		Health:    handler.NewHealthHandler(e.checks),
		Analytics: handler.NewAnalyticsHandler(e.analytics, e.link, e.auth, []string{frontendURL}),
		Validator:       e.auth,
		VerifyRateLimit: verifyLimit,
		ForgotRateLimit: ratelimit.Middleware(ratelimit.NewRedisLimiter(rdb, "rl:"),
			ratelimit.ForgotPasswordRules(3, time.Hour), ratelimit.Options{Message: "Too many reset requests. Try again later."}),
		ResetRateLimit: ratelimit.Middleware(ratelimit.NewRedisLimiter(rdb, "rl:"),
			ratelimit.ResetPasswordRules(3, 15*time.Minute), ratelimit.Options{Message: "Too many attempts. Try again later."}),
		AuthRateLimit: ratelimit.Middleware(ratelimit.NewRedisLimiter(rdb, "rl:"),
			ratelimit.AuthRules(orDefault(e.authLimit, 1000), time.Hour), ratelimit.Options{}),
		RefreshRateLimit: ratelimit.Middleware(ratelimit.NewRedisLimiter(rdb, "rl:"),
			ratelimit.RefreshRules(orDefault(e.refreshLimit, 1000), time.Hour), ratelimit.Options{FailOpen: true}),
		PublicRateLimit: ratelimit.Middleware(ratelimit.NewRedisLimiter(rdb, "rl:"),
			ratelimit.PublicRules(orDefault(e.publicLimit, 1000), time.Hour), ratelimit.Options{FailOpen: true}),
		UserRateLimit: ratelimit.Middleware(ratelimit.NewRedisLimiter(rdb, "rl:"),
			ratelimit.UserRules(orDefault(e.userLimit, 1000), time.Hour), ratelimit.Options{FailOpen: true}),
	}, router.Options{CORSOrigins: []string{frontendURL}})
	if err != nil {
		t.Fatal(err)
	}
	return engine
}

func (e *testEnv) do(t *testing.T, method, path, body string, opts ...func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	engine := e.engine(t)

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for _, opt := range opts {
		opt(req)
	}
	w := httptest.NewRecorder()
	engine.ServeHTTP(w, req)
	return w
}

func bearer(r *http.Request) { r.Header.Set("Authorization", "Bearer valid-token") }
func apiKey(r *http.Request) { r.Header.Set("X-API-Key", "lf_sk_valid") }
func withCookie(name, value string) func(*http.Request) {
	return func(r *http.Request) { r.AddCookie(&http.Cookie{Name: name, Value: value}) }
}

type envelope struct {
	Success bool                `json:"success"`
	Data    json.RawMessage     `json:"data"`
	Meta    *response.Meta      `json:"meta"`
	Error   *response.ErrorBody `json:"error"`
}

func decode(t *testing.T, w *httptest.ResponseRecorder) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid json body %q: %v", w.Body.String(), err)
	}
	return env
}

func decodeData[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	env := decode(t, w)
	if !env.Success {
		t.Fatalf("want success, got %s", w.Body.String())
	}
	if err := json.Unmarshal(env.Data, &v); err != nil {
		t.Fatalf("invalid data %s: %v", env.Data, err)
	}
	return v
}

func assertError(t *testing.T, w *httptest.ResponseRecorder, wantStatus int, wantCode string) {
	t.Helper()
	if w.Code != wantStatus {
		t.Fatalf("status = %d, want %d (body %s)", w.Code, wantStatus, w.Body.String())
	}
	env := decode(t, w)
	if env.Success || env.Error == nil || env.Error.Code != wantCode {
		t.Fatalf("error code = %+v, want %s", env.Error, wantCode)
	}
}

func findCookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

var testTokens = &authpb.TokenPair{AccessToken: "new-access", RefreshToken: "new-refresh"}

// Auth

func TestRegister(t *testing.T) {
	env := newEnv()
	var gotMD metadata.MD
	env.auth.register = func(ctx context.Context, r *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
		gotMD, _ = metadata.FromOutgoingContext(ctx)
		return &authpb.RegisterResponse{User: &authpb.User{Id: "user-1", Name: r.Name, Email: r.Email}, Tokens: testTokens}, nil
	}

	w := env.do(t, "POST", "/api/v1/auth/register", `{"name":"Dea","email":"dea@example.com","password":"password123"}`,
		func(r *http.Request) { r.Header.Set("User-Agent", "Mozilla/5.0 test") })

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	data := decodeData[map[string]any](t, w)
	user := data["user"].(map[string]any)
	if user["id"] != "user-1" || data["access_token"] != "new-access" || data["refresh_token"] != "new-refresh" {
		t.Errorf("unexpected data: %v", data)
	}
	if ua := gotMD.Get("x-user-agent"); len(ua) == 0 || ua[0] != "Mozilla/5.0 test" {
		t.Errorf("user agent not forwarded via metadata: %v", gotMD)
	}
	if ip := gotMD.Get("x-forwarded-for"); len(ip) == 0 || ip[0] == "" {
		t.Errorf("client ip not forwarded via metadata: %v", gotMD)
	}
}

func TestRegister_Errors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		rpcErr     error
		wantStatus int
		wantCode   string
		wantMsg    string
	}{
		{"missing body", "", nil, 400, "VALIDATION_ERROR", "Request body is required"},
		{"malformed json", `{"name":`, nil, 400, "VALIDATION_ERROR", "not valid JSON"},
		{"field validation uses json names", `{"email":"bad","password":"short"}`, nil, 400, "VALIDATION_ERROR",
			"name is required; email must be a valid email address; password must be at least 8 characters"},
		{"password over bcrypt limit", `{"name":"a","email":"a@b.com","password":"` + strings.Repeat("x", 73) + `"}`, nil, 400, "VALIDATION_ERROR", "at most 72"},
		{"email taken", `{"name":"a","email":"a@b.com","password":"password123"}`, status.Error(codes.AlreadyExists, "email already exists"), 409, "EMAIL_ALREADY_EXISTS", "email already exists"},
		{"auth service down", `{"name":"a","email":"a@b.com","password":"password123"}`, status.Error(codes.Unavailable, "dial failed"), 503, "SERVICE_UNAVAILABLE", ""},
		{"breaker open", `{"name":"a","email":"a@b.com","password":"password123"}`, breaker.ErrOpen, 503, "SERVICE_UNAVAILABLE", ""},
		{"internal error hidden", `{"name":"a","email":"a@b.com","password":"password123"}`, status.Error(codes.Internal, "pq: secret detail"), 500, "INTERNAL_ERROR", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newEnv()
			env.auth.register = func(context.Context, *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
				return nil, tt.rpcErr
			}
			w := env.do(t, "POST", "/api/v1/auth/register", tt.body)
			assertError(t, w, tt.wantStatus, tt.wantCode)
			msg := decode(t, w).Error.Message
			if tt.wantMsg != "" && !strings.Contains(msg, tt.wantMsg) {
				t.Errorf("message = %q, want it to contain %q", msg, tt.wantMsg)
			}
			if strings.Contains(msg, "pq:") {
				t.Errorf("internal detail leaked: %q", msg)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	env := newEnv()
	env.auth.login = func(_ context.Context, r *authpb.LoginRequest) (*authpb.LoginResponse, error) {
		if r.Password != "password123" {
			return nil, status.Error(codes.Unauthenticated, "invalid credentials")
		}
		return &authpb.LoginResponse{User: &authpb.User{Id: "user-1"}, Tokens: testTokens}, nil
	}

	if w := env.do(t, "POST", "/api/v1/auth/login", `{"email":"a@b.com","password":"password123"}`); w.Code != 200 {
		t.Errorf("valid login: status %d", w.Code)
	}
	assertError(t, env.do(t, "POST", "/api/v1/auth/login", `{"email":"a@b.com","password":"wrong"}`), 401, "UNAUTHORIZED")
	assertError(t, env.do(t, "POST", "/api/v1/auth/login", `{"email":"a@b.com"}`), 400, "VALIDATION_ERROR")
}

func TestRefresh(t *testing.T) {
	newRefreshEnv := func(got *string) *testEnv {
		env := newEnv()
		env.auth.refreshToken = func(_ context.Context, r *authpb.RefreshTokenRequest) (*authpb.RefreshTokenResponse, error) {
			*got = r.RefreshToken
			switch r.RefreshToken {
			case "reused":
				return nil, status.Error(codes.PermissionDenied, "refresh token abuse detected; all sessions revoked")
			case "expired":
				return nil, status.Error(codes.Unauthenticated, "refresh token expired")
			}
			return &authpb.RefreshTokenResponse{Tokens: testTokens}, nil
		}
		return env
	}

	t.Run("from body, no cookies set", func(t *testing.T) {
		var got string
		w := newRefreshEnv(&got).do(t, "POST", "/api/v1/auth/refresh", `{"refresh_token":"old"}`)
		data := decodeData[map[string]string](t, w)
		if got != "old" || data["refresh_token"] != "new-refresh" {
			t.Errorf("got token %q, data %v", got, data)
		}
		if findCookie(w, "refresh_token") != nil {
			t.Error("body-based refresh must not set cookies")
		}
	})
	t.Run("from cookie rotates cookies and never returns tokens in the body", func(t *testing.T) {
		var got string
		w := newRefreshEnv(&got).do(t, "POST", "/api/v1/auth/refresh", "", withCookie("refresh_token", "old-cookie"))
		if w.Code != http.StatusNoContent || got != "old-cookie" || w.Body.Len() != 0 {
			t.Fatalf("status %d, token %q, body %q", w.Code, got, w.Body.String())
		}
		rc, ac := findCookie(w, "refresh_token"), findCookie(w, "access_token")
		if rc == nil || rc.Value != "new-refresh" || rc.Path != "/api/v1/auth" || !rc.HttpOnly || !rc.Secure || rc.SameSite != http.SameSiteLaxMode {
			t.Errorf("refresh cookie = %+v", rc)
		}
		if ac == nil || ac.Value != "new-access" || ac.Path != "/" || !ac.HttpOnly {
			t.Errorf("access cookie = %+v", ac)
		}
	})
	t.Run("missing token", func(t *testing.T) {
		var got string
		assertError(t, newRefreshEnv(&got).do(t, "POST", "/api/v1/auth/refresh", ""), 400, "VALIDATION_ERROR")
	})
	t.Run("reuse detection maps to 401", func(t *testing.T) {
		var got string
		assertError(t, newRefreshEnv(&got).do(t, "POST", "/api/v1/auth/refresh", `{"refresh_token":"reused"}`), 401, "UNAUTHORIZED")
	})
	t.Run("failed cookie refresh clears cookies", func(t *testing.T) {
		var got string
		w := newRefreshEnv(&got).do(t, "POST", "/api/v1/auth/refresh", "", withCookie("refresh_token", "expired"))
		assertError(t, w, 401, "UNAUTHORIZED")
		if c := findCookie(w, "refresh_token"); c == nil || c.MaxAge >= 0 {
			t.Errorf("refresh cookie not cleared: %+v", c)
		}
	})
}

func TestLogout(t *testing.T) {
	var revoked string
	env := newEnv()
	env.auth.logout = func(_ context.Context, r *authpb.LogoutRequest) (*authpb.LogoutResponse, error) {
		revoked = r.RefreshToken
		return &authpb.LogoutResponse{Success: true}, nil
	}

	assertError(t, env.do(t, "POST", "/api/v1/auth/logout", `{"refresh_token":"rt"}`), 401, "UNAUTHORIZED")

	w := env.do(t, "POST", "/api/v1/auth/logout", "", bearer, withCookie("refresh_token", "rt-cookie"))
	if w.Code != http.StatusNoContent || revoked != "rt-cookie" {
		t.Fatalf("status %d, revoked %q", w.Code, revoked)
	}
	if c := findCookie(w, "access_token"); c == nil || c.MaxAge >= 0 {
		t.Errorf("access cookie not cleared: %+v", c)
	}

	assertError(t, env.do(t, "POST", "/api/v1/auth/logout", "", bearer), 400, "VALIDATION_ERROR")
}

func TestMe(t *testing.T) {
	created := time.Date(2026, 7, 8, 7, 0, 0, 0, time.UTC)
	env := newEnv()
	env.auth.getUser = func(_ context.Context, r *authpb.GetUserRequest) (*authpb.GetUserResponse, error) {
		return &authpb.GetUserResponse{User: &authpb.User{Id: r.UserId, Name: "Dea", Email: "dea@example.com", CreatedAt: timestamppb.New(created)}}, nil
	}

	for name, auth := range map[string]func(*http.Request){
		"bearer header":       bearer,
		"access_token cookie": withCookie("access_token", "valid-token"),
	} {
		t.Run(name, func(t *testing.T) {
			data := decodeData[map[string]string](t, env.do(t, "GET", "/api/v1/auth/me", "", auth))
			if data["id"] != "user-1" || data["created_at"] != "2026-07-08T07:00:00Z" {
				t.Errorf("data = %v", data)
			}
		})
	}
	t.Run("api key rejected", func(t *testing.T) {
		assertError(t, env.do(t, "GET", "/api/v1/auth/me", "", apiKey), 401, "UNAUTHORIZED")
	})
}

func TestApiKeys(t *testing.T) {
	created := timestamppb.New(time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC))
	env := newEnv()
	env.auth.createApiKey = func(_ context.Context, r *authpb.CreateApiKeyRequest) (*authpb.CreateApiKeyResponse, error) {
		return &authpb.CreateApiKeyResponse{ApiKey: &authpb.ApiKey{Id: "key-1", Name: r.Name, CreatedAt: created}, RawKey: "lf_sk_raw"}, nil
	}
	env.auth.listApiKeys = func(context.Context, *authpb.ListApiKeysRequest) (*authpb.ListApiKeysResponse, error) {
		return &authpb.ListApiKeysResponse{}, nil
	}
	env.auth.deleteApiKey = func(_ context.Context, r *authpb.DeleteApiKeyRequest) (*authpb.DeleteApiKeyResponse, error) {
		if r.ApiKeyId != "key-1" || r.UserId != "user-1" {
			return nil, status.Error(codes.NotFound, "api key not found")
		}
		return &authpb.DeleteApiKeyResponse{Success: true}, nil
	}

	w := env.do(t, "POST", "/api/v1/auth/api-keys", `{"name":"CLI"}`, bearer)
	if w.Code != 201 {
		t.Fatalf("create status %d", w.Code)
	}
	if data := decodeData[map[string]string](t, w); data["key"] != "lf_sk_raw" || data["id"] != "key-1" {
		t.Errorf("create data = %v", data)
	}
	assertError(t, env.do(t, "POST", "/api/v1/auth/api-keys", `{}`, bearer), 400, "VALIDATION_ERROR")

	if body := env.do(t, "GET", "/api/v1/auth/api-keys", "", bearer).Body.String(); body != `{"success":true,"data":[]}` {
		t.Errorf("empty list must be [] not null, got %s", body)
	}

	if w := env.do(t, "DELETE", "/api/v1/auth/api-keys/key-1", "", bearer); w.Code != 204 {
		t.Errorf("delete status %d", w.Code)
	}
	assertError(t, env.do(t, "DELETE", "/api/v1/auth/api-keys/other", "", bearer), 404, "NOT_FOUND")
}

func TestGoogleOAuth(t *testing.T) {
	t.Run("disabled without credentials", func(t *testing.T) {
		env := newEnv()
		assertError(t, env.do(t, "GET", "/api/v1/auth/oauth/google", ""), 503, "SERVICE_UNAVAILABLE")
		assertError(t, env.do(t, "GET", "/api/v1/auth/oauth/google/callback?code=x&state=y", ""), 503, "SERVICE_UNAVAILABLE")
	})

	enabled := func() *testEnv {
		env := newEnv()
		env.google = handler.NewGoogleOAuth("client-id", "client-secret", "https://lf.test/callback")
		return env
	}

	t.Run("login redirects to google with state cookie", func(t *testing.T) {
		w := enabled().do(t, "GET", "/api/v1/auth/oauth/google", "")
		if w.Code != http.StatusFound {
			t.Fatalf("status %d", w.Code)
		}
		loc, _ := url.Parse(w.Header().Get("Location"))
		state := findCookie(w, "oauth_state")
		if loc.Host != "accounts.google.com" || loc.Query().Get("client_id") != "client-id" ||
			loc.Query().Get("redirect_uri") != "https://lf.test/callback" || loc.Query().Get("scope") != "openid email profile" {
			t.Errorf("unexpected redirect %s", loc)
		}
		if state == nil || len(state.Value) != 64 || loc.Query().Get("state") != state.Value || !state.HttpOnly {
			t.Errorf("state cookie %+v does not match redirect state %q", state, loc.Query().Get("state"))
		}
	})

	callbackTests := []struct {
		name   string
		query  string
		cookie string
		reason string
	}{
		{"user denied consent", "?error=access_denied", "s1", "access_denied"},
		{"state mismatch (CSRF)", "?code=c&state=attacker", "s1", "invalid_state"},
		{"missing state cookie", "?code=c&state=s1", "", "invalid_state"},
		{"missing code", "?state=s1", "s1", "invalid_state"},
	}
	for _, tt := range callbackTests {
		t.Run("callback: "+tt.name, func(t *testing.T) {
			env := enabled()
			env.auth.googleAuth = func(context.Context, *authpb.GoogleAuthRequest) (*authpb.GoogleAuthResponse, error) {
				t.Fatal("GoogleAuth must not be called")
				return nil, nil
			}
			var opts []func(*http.Request)
			if tt.cookie != "" {
				opts = append(opts, withCookie("oauth_state", tt.cookie))
			}
			w := env.do(t, "GET", "/api/v1/auth/oauth/google/callback"+tt.query, "", opts...)
			if w.Code != http.StatusFound || w.Header().Get("Location") != frontendURL+"/app/login?error="+tt.reason {
				t.Errorf("got %d → %s", w.Code, w.Header().Get("Location"))
			}
			if findCookie(w, "access_token") != nil {
				t.Error("tokens must not be issued")
			}
		})
	}
}

// Links

func testLink(alias string) *linkpb.Link {
	ts := timestamppb.New(time.Date(2026, 7, 8, 7, 0, 0, 0, time.UTC))
	return &linkpb.Link{Id: "link-1", OriginalUrl: "https://example.com", Alias: alias, ClickCount: 3, CreatedAt: ts, UpdatedAt: ts}
}

func TestCreateLink(t *testing.T) {
	var got *linkpb.CreateLinkRequest
	env := newEnv()
	env.link.createLink = func(_ context.Context, r *linkpb.CreateLinkRequest) (*linkpb.CreateLinkResponse, error) {
		got = r
		return &linkpb.CreateLinkResponse{Link: testLink("my-alias")}, nil
	}

	w := env.do(t, "POST", "/api/v1/links", `{"original_url":"https://example.com","alias":"my-alias","expired_at":"2099-01-01T00:00:00Z"}`, apiKey)
	if w.Code != 201 {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	data := decodeData[map[string]any](t, w)
	if data["short_url"] != baseURL+"/my-alias" || data["alias"] != "my-alias" || data["expired_at"] != nil {
		t.Errorf("data = %v", data)
	}
	if got.UserId != "user-1" || got.Alias != "my-alias" || got.ExpiredAt.AsTime().Year() != 2099 {
		t.Errorf("rpc request = %+v", got)
	}
}

func TestCreateLink_Errors(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		rpcErr     error
		wantStatus int
		wantCode   string
	}{
		{"missing url", `{}`, nil, 400, "VALIDATION_ERROR"},
		{"not an http url", `{"original_url":"ftp://x"}`, nil, 400, "VALIDATION_ERROR"},
		{"expiry in the past", `{"original_url":"https://a.com","expired_at":"2020-01-01T00:00:00Z"}`, nil, 400, "VALIDATION_ERROR"},
		{"blocked domain", `{"original_url":"https://evil.com"}`, status.Error(codes.InvalidArgument, "url domain is blocked"), 400, "MALICIOUS_URL"},
		{"reserved alias", `{"original_url":"https://a.com","alias":"api"}`, status.Error(codes.InvalidArgument, "alias 'api' is reserved"), 400, "VALIDATION_ERROR"},
		{"alias taken", `{"original_url":"https://a.com","alias":"x-y-z"}`, status.Error(codes.AlreadyExists, "alias is already taken"), 409, "ALIAS_ALREADY_TAKEN"},
		{"link service down", `{"original_url":"https://a.com"}`, status.Error(codes.Unavailable, "down"), 503, "SERVICE_UNAVAILABLE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newEnv()
			env.link.createLink = func(context.Context, *linkpb.CreateLinkRequest) (*linkpb.CreateLinkResponse, error) {
				return nil, tt.rpcErr
			}
			assertError(t, env.do(t, "POST", "/api/v1/links", tt.body, bearer), tt.wantStatus, tt.wantCode)
		})
	}
}

func TestLinks_RequireAuth(t *testing.T) {
	env := newEnv()
	assertError(t, env.do(t, "GET", "/api/v1/links", ""), 401, "UNAUTHORIZED")
	assertError(t, env.do(t, "POST", "/api/v1/links", `{"original_url":"https://a.com"}`), 401, "UNAUTHORIZED")
}

func TestListLinks(t *testing.T) {
	var got *linkpb.ListLinksRequest
	env := newEnv()
	env.link.listLinks = func(_ context.Context, r *linkpb.ListLinksRequest) (*linkpb.ListLinksResponse, error) {
		got = r
		return &linkpb.ListLinksResponse{Links: []*linkpb.Link{testLink("a"), testLink("b")}, Total: 42}, nil
	}

	w := env.do(t, "GET", "/api/v1/links?page=2&limit=500", "", bearer)
	env2 := decode(t, w)
	if got.Page != 2 || got.Limit != 100 {
		t.Errorf("rpc page=%d limit=%d, want 2 and capped 100", got.Page, got.Limit)
	}
	if env2.Meta == nil || *env2.Meta != (response.Meta{Page: 2, Limit: 100, Total: 42}) {
		t.Errorf("meta = %+v", env2.Meta)
	}

	decode(t, env.do(t, "GET", "/api/v1/links", "", bearer))
	if got.Page != 1 || got.Limit != 20 {
		t.Errorf("defaults page=%d limit=%d, want 1 and 20", got.Page, got.Limit)
	}

	for _, q := range []string{"page=0", "page=abc", "limit=-1"} {
		assertError(t, env.do(t, "GET", "/api/v1/links?"+q, "", bearer), 400, "VALIDATION_ERROR")
	}
}

func TestGetUpdateDeleteLink(t *testing.T) {
	env := newEnv()
	env.link.getLink = func(_ context.Context, r *linkpb.GetLinkRequest) (*linkpb.GetLinkResponse, error) {
		if r.LinkId != "link-1" || r.UserId != "user-1" {
			return nil, status.Error(codes.NotFound, "link not found")
		}
		return &linkpb.GetLinkResponse{Link: testLink("a")}, nil
	}
	var update *linkpb.UpdateLinkRequest
	env.link.updateLink = func(_ context.Context, r *linkpb.UpdateLinkRequest) (*linkpb.UpdateLinkResponse, error) {
		update = r
		return &linkpb.UpdateLinkResponse{Link: testLink("new-alias")}, nil
	}
	env.link.deleteLink = func(_ context.Context, r *linkpb.DeleteLinkRequest) (*linkpb.DeleteLinkResponse, error) {
		if r.LinkId != "link-1" {
			return nil, status.Error(codes.NotFound, "link not found")
		}
		return &linkpb.DeleteLinkResponse{Success: true}, nil
	}

	if data := decodeData[map[string]any](t, env.do(t, "GET", "/api/v1/links/link-1", "", bearer)); data["click_count"] != float64(3) {
		t.Errorf("get data = %v", data)
	}
	assertError(t, env.do(t, "GET", "/api/v1/links/missing", "", bearer), 404, "LINK_NOT_FOUND")

	w := env.do(t, "PUT", "/api/v1/links/link-1", `{"alias":"new-alias","expired_at":null}`, bearer)
	if w.Code != 200 || update.Alias != "new-alias" || update.ExpiredAt != nil || update.LinkId != "link-1" {
		t.Errorf("update status %d, rpc %+v", w.Code, update)
	}
	assertError(t, env.do(t, "PUT", "/api/v1/links/link-1", `{"original_url":"not a url"}`, bearer), 400, "VALIDATION_ERROR")

	if w := env.do(t, "DELETE", "/api/v1/links/link-1", "", apiKey); w.Code != 204 {
		t.Errorf("delete status %d", w.Code)
	}
	assertError(t, env.do(t, "DELETE", "/api/v1/links/missing", "", apiKey), 404, "LINK_NOT_FOUND")
}

// Redirect

func TestRedirect(t *testing.T) {
	env := newEnv()
	env.link.resolveAlias = func(_ context.Context, r *linkpb.ResolveAliasRequest) (*linkpb.ResolveAliasResponse, error) {
		switch r.Alias {
		case "ok":
			return &linkpb.ResolveAliasResponse{OriginalUrl: "https://example.com/dest"}, nil
		case "old":
			return &linkpb.ResolveAliasResponse{OriginalUrl: "https://example.com", IsExpired: true}, nil
		case "locked":
			return &linkpb.ResolveAliasResponse{OriginalUrl: "https://secret.com", HasPassword: true}, nil
		case "down":
			return nil, breaker.ErrOpen
		}
		return nil, status.Error(codes.NotFound, "link not found")
	}

	w := env.do(t, "GET", "/ok", "")
	if w.Code != http.StatusFound || w.Header().Get("Location") != "https://example.com/dest" {
		t.Errorf("got %d → %s", w.Code, w.Header().Get("Location"))
	}
	assertError(t, env.do(t, "GET", "/old", ""), 410, "LINK_EXPIRED")
	assertError(t, env.do(t, "GET", "/missing", ""), 404, "LINK_NOT_FOUND")
	assertError(t, env.do(t, "GET", "/down", ""), 503, "SERVICE_UNAVAILABLE")

	w = env.do(t, "GET", "/locked", "")
	assertError(t, w, 200, "PASSWORD_REQUIRED")
	if strings.Contains(w.Body.String(), "secret.com") {
		t.Error("destination of a password-protected link must not be revealed")
	}
}

func TestVerifyPassword(t *testing.T) {
	env := newEnv()
	env.link.verifyLinkPassword = func(_ context.Context, r *linkpb.VerifyLinkPasswordRequest) (*linkpb.VerifyLinkPasswordResponse, error) {
		switch {
		case r.Alias == "old":
			return nil, status.Error(codes.FailedPrecondition, "link has expired")
		case r.Alias == "missing":
			return nil, status.Error(codes.NotFound, "link not found")
		case r.Password != "lock1234":
			return nil, status.Error(codes.Unauthenticated, "invalid password")
		}
		return &linkpb.VerifyLinkPasswordResponse{IsValid: true, OriginalUrl: "https://secret.com"}, nil
	}

	if data := decodeData[map[string]string](t, env.do(t, "POST", "/locked/verify", `{"password":"lock1234"}`)); data["redirect_url"] != "https://secret.com" {
		t.Errorf("data = %v", data)
	}
	assertError(t, env.do(t, "POST", "/locked/verify", `{"password":"nope"}`), 401, "UNAUTHORIZED")
	assertError(t, env.do(t, "POST", "/locked/verify", `{}`), 400, "VALIDATION_ERROR")
	assertError(t, env.do(t, "POST", "/old/verify", `{"password":"lock1234"}`), 410, "LINK_EXPIRED")
	assertError(t, env.do(t, "POST", "/missing/verify", `{"password":"lock1234"}`), 404, "LINK_NOT_FOUND")
}

func TestVerifyPassword_InvalidResultWithoutError(t *testing.T) {
	env := newEnv()
	env.link.verifyLinkPassword = func(context.Context, *linkpb.VerifyLinkPasswordRequest) (*linkpb.VerifyLinkPasswordResponse, error) {
		return &linkpb.VerifyLinkPasswordResponse{IsValid: false, OriginalUrl: "https://secret.com"}, nil
	}
	w := env.do(t, "POST", "/locked/verify", `{"password":"x"}`)
	assertError(t, w, 401, "UNAUTHORIZED")
	if strings.Contains(w.Body.String(), "secret.com") {
		t.Error("destination leaked on invalid password")
	}
}

func TestRedirect_ForwardsClientInfoForAnalytics(t *testing.T) {
	env := newEnv()
	var md metadata.MD
	env.link.resolveAlias = func(ctx context.Context, _ *linkpb.ResolveAliasRequest) (*linkpb.ResolveAliasResponse, error) {
		md, _ = metadata.FromOutgoingContext(ctx)
		return &linkpb.ResolveAliasResponse{OriginalUrl: "https://example.com"}, nil
	}

	env.do(t, "GET", "/promo", "", func(r *http.Request) {
		r.Header.Set("User-Agent", "Mozilla/5.0 (iPhone)")
		r.Header.Set("Referer", "https://twitter.com/")
		r.RemoteAddr = "203.0.113.7:51234"
	})

	want := map[string]string{"x-forwarded-for": "203.0.113.7", "x-user-agent": "Mozilla/5.0 (iPhone)", "x-referer": "https://twitter.com/"}
	for key, val := range want {
		if got := md.Get(key); len(got) == 0 || got[0] != val {
			t.Errorf("metadata %s = %v, want %q", key, got, val)
		}
	}
}

// Ops & routing

func TestHealth(t *testing.T) {
	env := newEnv()
	if w := env.do(t, "GET", "/healthz", ""); w.Code != 200 || w.Body.String() != `{"status":"ok"}` {
		t.Errorf("healthz: %d %s", w.Code, w.Body.String())
	}

	env.checks["auth"] = func(context.Context) error { return nil }
	env.checks["redis"] = func(context.Context) error { return nil }
	if w := env.do(t, "GET", "/readyz", ""); w.Code != 200 {
		t.Errorf("readyz all healthy: %d %s", w.Code, w.Body.String())
	}

	env.checks["link"] = func(context.Context) error { return errors.New("connection refused") }
	w := env.do(t, "GET", "/readyz", "")
	var body struct {
		Status string            `json:"status"`
		Checks map[string]string `json:"checks"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	if w.Code != 503 || body.Status != "unavailable" || body.Checks["link"] != "connection refused" || body.Checks["auth"] != "ok" {
		t.Errorf("readyz with failing link: %d %+v", w.Code, body)
	}
}

func TestReadyz_RespectsTimeout(t *testing.T) {
	env := newEnv()
	env.checks["slow"] = func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}
	start := time.Now()
	w := env.do(t, "GET", "/readyz", "")
	if w.Code != 503 || time.Since(start) > 3*time.Second {
		t.Errorf("slow dependency must fail readiness within the timeout: %d after %v", w.Code, time.Since(start))
	}
}

func TestRouting(t *testing.T) {
	env := newEnv()
	assertError(t, env.do(t, "GET", "/api/v1/unknown", ""), 404, "NOT_FOUND")
	assertError(t, env.do(t, "GET", "/some/deep/path", ""), 404, "NOT_FOUND")

	t.Run("CORS preflight allows frontend with credentials", func(t *testing.T) {
		w := env.do(t, "OPTIONS", "/api/v1/links", "", func(r *http.Request) {
			r.Header.Set("Origin", frontendURL)
			r.Header.Set("Access-Control-Request-Method", "POST")
		})
		if w.Code != 204 || w.Header().Get("Access-Control-Allow-Origin") != frontendURL || w.Header().Get("Access-Control-Allow-Credentials") != "true" {
			t.Errorf("preflight: %d %v", w.Code, w.Header())
		}
	})
	t.Run("CORS rejects unknown origin", func(t *testing.T) {
		w := env.do(t, "OPTIONS", "/api/v1/links", "", func(r *http.Request) {
			r.Header.Set("Origin", "https://evil.test")
			r.Header.Set("Access-Control-Request-Method", "POST")
		})
		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Errorf("unknown origin allowed: %v", w.Header())
		}
	})
}

// roundTripFunc mencegat request ke token endpoint Google.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func googleTokenEndpoint(t *testing.T, status int, body string) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.String() != "https://oauth2.googleapis.com/token" {
			t.Errorf("unexpected token url %s", r.URL)
		}
		_ = r.ParseForm()
		if r.PostForm.Get("code") != "auth-code" || r.PostForm.Get("client_secret") != "client-secret" ||
			r.PostForm.Get("grant_type") != "authorization_code" {
			t.Errorf("unexpected token request form %v", r.PostForm)
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
	})}
}

func TestGoogleCallback(t *testing.T) {
	tests := []struct {
		name         string
		status       int
		body         string
		authErr      error
		wantLocation string
	}{
		{"success sets cookies and redirects to dashboard", 200, `{"id_token":"google-id-token"}`, nil, frontendURL + "/app/dashboard"},
		{"google rejects code", 400, `{"error":"invalid_grant"}`, nil, frontendURL + "/app/login?error=exchange_failed"},
		{"response without id_token", 200, `{"access_token":"x"}`, nil, frontendURL + "/app/login?error=exchange_failed"},
		{"auth service rejects id token", 200, `{"id_token":"google-id-token"}`, status.Error(codes.Unauthenticated, "email is not verified"), frontendURL + "/app/login?error=auth_failed"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newEnv()
			env.google = handler.NewGoogleOAuth("client-id", "client-secret", "https://lf.test/callback")
			env.google.HTTPClient = googleTokenEndpoint(t, tt.status, tt.body)
			env.auth.googleAuth = func(_ context.Context, r *authpb.GoogleAuthRequest) (*authpb.GoogleAuthResponse, error) {
				if r.IdToken != "google-id-token" {
					t.Errorf("id token = %q", r.IdToken)
				}
				if tt.authErr != nil {
					return nil, tt.authErr
				}
				return &authpb.GoogleAuthResponse{User: &authpb.User{Id: "user-1"}, Tokens: testTokens}, nil
			}

			w := env.do(t, "GET", "/api/v1/auth/oauth/google/callback?code=auth-code&state=s1", "", withCookie("oauth_state", "s1"))
			if w.Code != http.StatusFound || w.Header().Get("Location") != tt.wantLocation {
				t.Fatalf("got %d → %s", w.Code, w.Header().Get("Location"))
			}
			if c := findCookie(w, "oauth_state"); c == nil || c.MaxAge >= 0 {
				t.Errorf("state cookie must be cleared after callback: %+v", c)
			}
			access := findCookie(w, "access_token")
			if tt.wantLocation == frontendURL+"/app/dashboard" {
				if access == nil || access.Value != "new-access" || findCookie(w, "refresh_token").Value != "new-refresh" {
					t.Errorf("auth cookies not set: %v", w.Result().Cookies())
				}
				if strings.Contains(w.Header().Get("Location"), "new-access") {
					t.Error("tokens must never be put in the redirect URL")
				}
			} else if access != nil {
				t.Error("tokens must not be issued on failure")
			}
		})
	}
}
