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
