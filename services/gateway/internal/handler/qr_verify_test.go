package handler_test

import (
	"bytes"
	"context"
	"encoding/xml"
	"image/png"
	"net/http"
	"strconv"
	"strings"
	"testing"

	authpb "github.com/deaprima/linkforge/services/shared/proto/auth"
	linkpb "github.com/deaprima/linkforge/services/shared/proto/link"
	qrcode "github.com/skip2/go-qrcode"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// QR code

func qrEnv() *testEnv {
	env := newEnv()
	env.link.getLink = func(_ context.Context, r *linkpb.GetLinkRequest) (*linkpb.GetLinkResponse, error) {
		if r.LinkId != "link-1" || r.UserId != "user-1" {
			return nil, status.Error(codes.NotFound, "link not found")
		}
		return &linkpb.GetLinkResponse{Link: testLink("my-alias")}, nil
	}
	return env
}

func TestQR_PNG(t *testing.T) {
	w := qrEnv().do(t, "GET", "/api/v1/links/link-1/qr?size=300", "", apiKey)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("got %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	img, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil {
		t.Fatalf("not a valid PNG: %v", err)
	}
	if b := img.Bounds(); b.Dx() != 300 || b.Dy() != 300 {
		t.Errorf("size = %dx%d, want 300x300", b.Dx(), b.Dy())
	}

	// Isi QR = short_url link (encoding deterministik, jadi bisa dibandingkan byte-per-byte)
	want, _ := qrcode.Encode(baseURL+"/my-alias", qrcode.Medium, 300)
	if !bytes.Equal(w.Body.Bytes(), want) {
		t.Error("QR must encode the link's short_url")
	}
	if cc := w.Header().Get("Cache-Control"); cc != "private, max-age=3600" {
		t.Errorf("Cache-Control = %q", cc)
	}
	if cd := w.Header().Get("Content-Disposition"); cd != `inline; filename="my-alias.png"` {
		t.Errorf("Content-Disposition = %q", cd)
	}
}

func TestQR_DefaultSizeIs256(t *testing.T) {
	w := qrEnv().do(t, "GET", "/api/v1/links/link-1/qr", "", bearer)
	img, err := png.Decode(bytes.NewReader(w.Body.Bytes()))
	if err != nil || img.Bounds().Dx() != 256 {
		t.Fatalf("default size: err=%v", err)
	}
}

func TestQR_SVG(t *testing.T) {
	w := qrEnv().do(t, "GET", "/api/v1/links/link-1/qr?format=svg&size=512", "", bearer)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/svg+xml" {
		t.Fatalf("got %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	var svg struct {
		XMLName xml.Name `xml:"svg"`
		Width   string   `xml:"width,attr"`
		ViewBox string   `xml:"viewBox,attr"`
		G       struct {
			Rects []struct{} `xml:"rect"`
		} `xml:"g"`
	}
	if err := xml.Unmarshal(w.Body.Bytes(), &svg); err != nil {
		t.Fatalf("invalid SVG: %v", err)
	}
	if svg.Width != "512" || len(svg.G.Rects) == 0 {
		t.Errorf("svg width=%q rects=%d", svg.Width, len(svg.G.Rects))
	}

	// Dimensi viewBox = jumlah modul QR (termasuk quiet zone) dari isi yang benar
	q, _ := qrcode.New(baseURL+"/my-alias", qrcode.Medium)
	n := strconv.Itoa(len(q.Bitmap()))
	if svg.ViewBox != "0 0 "+n+" "+n {
		t.Errorf("viewBox = %q, want %d modules", svg.ViewBox, len(q.Bitmap()))
	}
}

func TestQR_Errors(t *testing.T) {
	tests := []struct {
		name, path string
		wantStatus int
		wantCode   string
	}{
		{"size too small", "/api/v1/links/link-1/qr?size=10", 400, "VALIDATION_ERROR"},
		{"size too large", "/api/v1/links/link-1/qr?size=5000", 400, "VALIDATION_ERROR"},
		{"size not a number", "/api/v1/links/link-1/qr?size=big", 400, "VALIDATION_ERROR"},
		{"unknown format", "/api/v1/links/link-1/qr?format=gif", 400, "VALIDATION_ERROR"},
		{"not the owner", "/api/v1/links/someone-elses/qr", 404, "LINK_NOT_FOUND"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertError(t, qrEnv().do(t, "GET", tt.path, "", bearer), tt.wantStatus, tt.wantCode)
		})
	}
	assertError(t, qrEnv().do(t, "GET", "/api/v1/links/link-1/qr", ""), 401, "UNAUTHORIZED")
}

func TestQR_BoundarySizes(t *testing.T) {
	for _, size := range []string{"64", "1024"} {
		if w := qrEnv().do(t, "GET", "/api/v1/links/link-1/qr?size="+size, "", bearer); w.Code != 200 {
			t.Errorf("size=%s: %d", size, w.Code)
		}
	}
}

// Brute-force protection POST /:alias/verify

func verifyEnv() *testEnv {
	env := newEnv()
	env.link.verifyLinkPassword = func(_ context.Context, r *linkpb.VerifyLinkPasswordRequest) (*linkpb.VerifyLinkPasswordResponse, error) {
		if r.Password != "lock1234" {
			return nil, status.Error(codes.Unauthenticated, "invalid password")
		}
		return &linkpb.VerifyLinkPasswordResponse{IsValid: true, OriginalUrl: "https://secret.com"}, nil
	}
	return env
}

func fromIP(ip string) func(*http.Request) {
	return func(r *http.Request) { r.RemoteAddr = ip + ":40000" }
}

func TestVerify_BruteForceIsBlockedPerAlias(t *testing.T) {
	env := verifyEnv()
	calls := 0
	inner := env.link.verifyLinkPassword
	env.link.verifyLinkPassword = func(ctx context.Context, r *linkpb.VerifyLinkPasswordRequest) (*linkpb.VerifyLinkPasswordResponse, error) {
		calls++
		return inner(ctx, r)
	}

	for i := 1; i <= 5; i++ {
		w := env.do(t, "POST", "/locked/verify", `{"password":"guess`+strconv.Itoa(i)+`"}`, fromIP("198.51.100.9"))
		assertError(t, w, 401, "UNAUTHORIZED")
		if rem := w.Header().Get("X-RateLimit-Remaining"); rem != strconv.Itoa(5-i) {
			t.Errorf("attempt %d: X-RateLimit-Remaining = %q, want %d", i, rem, 5-i)
		}
	}

	// Tebakan ke-6 ditolak — bahkan password yang benar, dan Link Service tidak dipanggil
	w := env.do(t, "POST", "/locked/verify", `{"password":"lock1234"}`, fromIP("198.51.100.9"))
	assertError(t, w, 429, "RATE_LIMIT_EXCEEDED")
	if ra, _ := strconv.Atoi(w.Header().Get("Retry-After")); ra <= 0 || ra > 15*60 {
		t.Errorf("Retry-After = %q", w.Header().Get("Retry-After"))
	}
	if !strings.Contains(w.Body.String(), "Too many password attempts") || strings.Contains(w.Body.String(), "secret.com") {
		t.Errorf("body = %s", w.Body.String())
	}
	if calls != 5 {
		t.Errorf("link service called %d times, want 5 (blocked requests must not reach it)", calls)
	}

	// Pengunjung lain (IP berbeda) tetap bisa membuka link yang sama
	if w := env.do(t, "POST", "/locked/verify", `{"password":"lock1234"}`, fromIP("203.0.113.50")); w.Code != 200 {
		t.Errorf("other visitor locked out: %d %s", w.Code, w.Body.String())
	}
}

func TestVerify_BruteForceAcrossAliasesIsBlockedPerIP(t *testing.T) {
	env := verifyEnv()
	for i := 0; i < 20; i++ {
		alias := "link" + strconv.Itoa(i/4) // 4 percobaan per alias, masih di bawah batas per-alias
		if w := env.do(t, "POST", "/"+alias+"/verify", `{"password":"x"}`, fromIP("198.51.100.9")); w.Code == 429 {
			t.Fatalf("attempt %d blocked too early", i+1)
		}
	}
	assertError(t, env.do(t, "POST", "/fresh-alias/verify", `{"password":"x"}`, fromIP("198.51.100.9")), 429, "RATE_LIMIT_EXCEEDED")
}

func TestVerify_FailClosedWhenRedisDown(t *testing.T) {
	env := verifyEnv()
	env.redis.Close()

	w := env.do(t, "POST", "/locked/verify", `{"password":"lock1234"}`)
	assertError(t, w, 503, "SERVICE_UNAVAILABLE")
	if strings.Contains(w.Body.String(), "secret.com") {
		t.Error("destination must not be revealed when the limiter is down")
	}

	// Redirect link tanpa password tidak bergantung pada limiter
	env.link.resolveAlias = func(context.Context, *linkpb.ResolveAliasRequest) (*linkpb.ResolveAliasResponse, error) {
		return &linkpb.ResolveAliasResponse{OriginalUrl: "https://example.com"}, nil
	}
	if w := env.do(t, "GET", "/open-link", ""); w.Code != http.StatusFound {
		t.Errorf("public redirect must keep working without redis: %d", w.Code)
	}
}

// Prasyarat web app (Fase 5)

func browser(r *http.Request) {
	r.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
}

func TestRedirect_BrowserGoesToWebPages(t *testing.T) {
	env := newEnv()
	env.link.resolveAlias = func(_ context.Context, r *linkpb.ResolveAliasRequest) (*linkpb.ResolveAliasResponse, error) {
		switch r.Alias {
		case "locked":
			return &linkpb.ResolveAliasResponse{OriginalUrl: "https://secret.com", HasPassword: true}, nil
		case "old":
			return &linkpb.ResolveAliasResponse{OriginalUrl: "https://example.com", IsExpired: true}, nil
		case "down":
			return nil, status.Error(codes.Unavailable, "link service down")
		case "ok":
			return &linkpb.ResolveAliasResponse{OriginalUrl: "https://example.com/dest"}, nil
		}
		return nil, status.Error(codes.NotFound, "link not found")
	}

	tests := []struct {
		alias, wantLocation string
	}{
		{"locked", frontendURL + "/app/p/locked"},
		{"old", frontendURL + "/app/expired?alias=old"},
		{"missing", frontendURL + "/app/not-found?alias=missing"},
		{"ok", "https://example.com/dest"},
	}
	for _, tt := range tests {
		w := env.do(t, "GET", "/"+tt.alias, "", browser)
		if w.Code != http.StatusFound || w.Header().Get("Location") != tt.wantLocation {
			t.Errorf("%s: got %d → %q, want 302 → %q", tt.alias, w.Code, w.Header().Get("Location"), tt.wantLocation)
		}
		if strings.Contains(w.Header().Get("Location"), "secret.com") {
			t.Errorf("%s: destination of a password-protected link leaked", tt.alias)
		}
	}

	// Backend down tidak boleh menyamar jadi "link tidak ditemukan"
	assertError(t, env.do(t, "GET", "/down", "", browser), 503, "SERVICE_UNAVAILABLE")

	// Client API (tanpa Accept: text/html) tetap mendapat JSON
	assertError(t, env.do(t, "GET", "/locked", ""), 200, "PASSWORD_REQUIRED")
	assertError(t, env.do(t, "GET", "/missing", "", func(r *http.Request) { r.Header.Set("Accept", "application/json") }), 404, "LINK_NOT_FOUND")
}

func TestLoginAndRegisterSetSessionCookies(t *testing.T) {
	env := newEnv()
	env.auth.login = func(context.Context, *authpb.LoginRequest) (*authpb.LoginResponse, error) {
		return &authpb.LoginResponse{User: &authpb.User{Id: "user-1"}, Tokens: testTokens}, nil
	}
	env.auth.register = func(context.Context, *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
		return &authpb.RegisterResponse{User: &authpb.User{Id: "user-1"}, Tokens: testTokens}, nil
	}

	for path, body := range map[string]string{
		"/api/v1/auth/login":    `{"email":"a@b.com","password":"password123"}`,
		"/api/v1/auth/register": `{"name":"a","email":"a@b.com","password":"password123"}`,
	} {
		w := env.do(t, "POST", path, body)
		access, refresh := findCookie(w, "access_token"), findCookie(w, "refresh_token")
		if access == nil || access.Value != "new-access" || !access.HttpOnly || access.Path != "/" {
			t.Errorf("%s: access cookie = %+v", path, access)
		}
		if refresh == nil || refresh.Value != "new-refresh" || !refresh.HttpOnly || refresh.Path != "/api/v1/auth" || refresh.SameSite != http.SameSiteLaxMode {
			t.Errorf("%s: refresh cookie = %+v", path, refresh)
		}
		// Token tetap ada di body untuk client non-browser
		if !strings.Contains(w.Body.String(), `"access_token":"new-access"`) {
			t.Errorf("%s: body must still contain tokens: %s", path, w.Body.String())
		}
	}

	// Login gagal tidak memasang cookie
	env.auth.login = func(context.Context, *authpb.LoginRequest) (*authpb.LoginResponse, error) {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}
	if w := env.do(t, "POST", "/api/v1/auth/login", `{"email":"a@b.com","password":"wrong"}`); findCookie(w, "access_token") != nil {
		t.Error("failed login must not set cookies")
	}
}

func TestListApiKeysIncludesPrefix(t *testing.T) {
	env := newEnv()
	env.auth.listApiKeys = func(context.Context, *authpb.ListApiKeysRequest) (*authpb.ListApiKeysResponse, error) {
		return &authpb.ListApiKeysResponse{ApiKeys: []*authpb.ApiKey{{Id: "k1", Name: "CLI", KeyPrefix: "lf_sk_a3f9b2"}}}, nil
	}
	if body := env.do(t, "GET", "/api/v1/auth/api-keys", "", bearer).Body.String(); !strings.Contains(body, `"key_prefix":"lf_sk_a3f9b2"`) {
		t.Errorf("body = %s", body)
	}
}

func TestCookieOnlySessionModeNeverExposesTokens(t *testing.T) {
	env := newEnv()
	env.auth.login = func(context.Context, *authpb.LoginRequest) (*authpb.LoginResponse, error) {
		return &authpb.LoginResponse{User: &authpb.User{Id: "user-1", Email: "a@b.com"}, Tokens: testTokens}, nil
	}
	env.auth.register = func(context.Context, *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
		return &authpb.RegisterResponse{User: &authpb.User{Id: "user-1"}, Tokens: testTokens}, nil
	}
	cookieMode := func(r *http.Request) { r.Header.Set("X-Session-Mode", "cookie") }

	for path, body := range map[string]string{
		"/api/v1/auth/login":    `{"email":"a@b.com","password":"password123"}`,
		"/api/v1/auth/register": `{"name":"a","email":"a@b.com","password":"password123"}`,
	} {
		w := env.do(t, "POST", path, body, cookieMode)
		if strings.Contains(w.Body.String(), "new-access") || strings.Contains(w.Body.String(), "new-refresh") {
			t.Errorf("%s: tokens leaked into a JS-readable body: %s", path, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), `"user":{"id":"user-1"`) {
			t.Errorf("%s: body must contain the user: %s", path, w.Body.String())
		}
		if findCookie(w, "access_token") == nil || findCookie(w, "refresh_token") == nil {
			t.Errorf("%s: session cookies must still be set", path)
		}
	}
}
