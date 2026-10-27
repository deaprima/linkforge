package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func init() { gin.SetMode(gin.TestMode) }

func newTestLimiter(t *testing.T) (*RedisLimiter, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { rdb.Close() })
	return NewRedisLimiter(rdb, "test:"), mr
}

func TestAllow_FixedWindow(t *testing.T) {
	l, mr := newTestLimiter(t)
	ctx := context.Background()
	rule := Rule{Key: "k", Limit: 3, Window: time.Minute}

	for i, wantRemaining := range []int{2, 1, 0} {
		res, err := l.Allow(ctx, rule)
		if err != nil || !res.Allowed || res.Remaining != wantRemaining || res.Limit != 3 {
			t.Fatalf("request %d: %+v err=%v", i+1, res, err)
		}
	}

	res, _ := l.Allow(ctx, rule)
	if res.Allowed || res.Remaining != 0 || res.RetryAfter <= 0 || res.RetryAfter > time.Minute {
		t.Fatalf("4th request must be blocked with a retry-after within the window: %+v", res)
	}

	// Terus mencoba saat diblokir tidak memperpanjang window (fixed window)
	mr.FastForward(30 * time.Second)
	_, _ = l.Allow(ctx, rule)
	mr.FastForward(31 * time.Second)

	res, _ = l.Allow(ctx, rule)
	if !res.Allowed || res.Remaining != 2 {
		t.Errorf("new window must start fresh: %+v", res)
	}
}

func TestAllow_KeysAreIndependentAndPrefixed(t *testing.T) {
	l, mr := newTestLimiter(t)
	ctx := context.Background()
	a := Rule{Key: "a", Limit: 1, Window: time.Minute}
	b := Rule{Key: "b", Limit: 1, Window: time.Minute}

	_, _ = l.Allow(ctx, a)
	if res, _ := l.Allow(ctx, b); !res.Allowed {
		t.Error("exhausting key a must not affect key b")
	}
	if !mr.Exists("test:a") || mr.Exists("a") {
		t.Errorf("keys must be stored with prefix, got %v", mr.Keys())
	}
	if ttl := mr.TTL("test:a"); ttl <= 0 || ttl > time.Minute {
		t.Errorf("key must expire with the window, ttl=%v", ttl)
	}
}

func TestAllow_MultipleRules(t *testing.T) {
	l, _ := newTestLimiter(t)
	ctx := context.Background()
	strict := Rule{Key: "strict", Limit: 2, Window: time.Minute}
	loose := Rule{Key: "loose", Limit: 10, Window: time.Hour}

	res, _ := l.Allow(ctx, strict, loose)
	if !res.Allowed || res.Limit != 2 || res.Remaining != 1 {
		t.Errorf("headers must reflect the rule closest to its limit: %+v", res)
	}

	_, _ = l.Allow(ctx, strict, loose)
	res, _ = l.Allow(ctx, strict, loose)
	if res.Allowed || res.Limit != 2 {
		t.Errorf("must be blocked by the strict rule: %+v", res)
	}

	// Rule longgar ikut terhitung walau ditolak oleh rule ketat (semua rule dicatat atomik)
	res, _ = l.Allow(ctx, loose)
	if res.Remaining != 6 {
		t.Errorf("loose rule remaining = %d, want 6 (4 requests counted)", res.Remaining)
	}
}

func TestAllow_BlockedByLongestWindow(t *testing.T) {
	l, _ := newTestLimiter(t)
	ctx := context.Background()
	short := Rule{Key: "short", Limit: 1, Window: time.Minute}
	long := Rule{Key: "long", Limit: 1, Window: time.Hour}

	_, _ = l.Allow(ctx, short, long)
	res, _ := l.Allow(ctx, short, long)
	if res.Allowed || res.RetryAfter <= time.Minute {
		t.Errorf("when several rules are exceeded, retry-after must be the longest: %+v", res)
	}
}

func TestAllow_NoRulesAndRedisDown(t *testing.T) {
	l, mr := newTestLimiter(t)
	if res, err := l.Allow(context.Background()); err != nil || !res.Allowed {
		t.Errorf("no rules = allowed: %+v %v", res, err)
	}
	mr.Close()
	if _, err := l.Allow(context.Background(), Rule{Key: "k", Limit: 1, Window: time.Minute}); err == nil {
		t.Error("want error when redis is unavailable")
	}
}

// Middleware

func serve(t *testing.T, mw gin.HandlerFunc, opts ...func(*http.Request)) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.POST("/:alias/verify", mw, func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	req := httptest.NewRequest(http.MethodPost, "/promo/verify", nil)
	req.RemoteAddr = "203.0.113.7:1234"
	for _, o := range opts {
		o(req)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestMiddleware_HeadersAnd429(t *testing.T) {
	l, _ := newTestLimiter(t)
	mw := Middleware(l, VerifyPasswordRules(2, 10, 15*time.Minute), Options{Message: "Too many password attempts."})

	w := serve(t, mw)
	if w.Code != 200 || w.Header().Get("X-RateLimit-Limit") != "2" || w.Header().Get("X-RateLimit-Remaining") != "1" {
		t.Fatalf("first: %d %v", w.Code, w.Header())
	}
	reset := w.Header().Get("X-RateLimit-Reset")
	if reset == "" || reset < "1700000000" {
		t.Errorf("X-RateLimit-Reset must be a unix timestamp, got %q", reset)
	}

	serve(t, mw)
	w = serve(t, mw)
	if w.Code != http.StatusTooManyRequests || w.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("third: %d %v", w.Code, w.Header())
	}
	if ra := w.Header().Get("Retry-After"); ra == "" || ra == "0" {
		t.Errorf("Retry-After = %q", ra)
	}
	if !strings.Contains(w.Body.String(), `"code":"RATE_LIMIT_EXCEEDED"`) || !strings.Contains(w.Body.String(), "Too many password attempts.") {
		t.Errorf("body = %s", w.Body.String())
	}
}

func TestMiddleware_FailClosedAndFailOpen(t *testing.T) {
	l, mr := newTestLimiter(t)
	rules := VerifyPasswordRules(5, 20, time.Minute)
	mr.Close()

	if w := serve(t, Middleware(l, rules, Options{})); w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "SERVICE_UNAVAILABLE") {
		t.Errorf("fail-closed: %d %s", w.Code, w.Body.String())
	}
	if w := serve(t, Middleware(l, rules, Options{FailOpen: true})); w.Code != 200 {
		t.Errorf("fail-open: %d %s", w.Code, w.Body.String())
	}
}

func TestVerifyPasswordRules(t *testing.T) {
	l, _ := newTestLimiter(t)
	mw := Middleware(l, VerifyPasswordRules(2, 3, time.Minute), Options{})
	fromIP := func(ip string) func(*http.Request) { return func(r *http.Request) { r.RemoteAddr = ip + ":1" } }
	path := func(p string) func(*http.Request) { return func(r *http.Request) { r.URL.Path = p } }

	// Per IP+alias: 2 percobaan untuk /promo dari IP A
	serve(t, mw, fromIP("10.0.0.1"))
	serve(t, mw, fromIP("10.0.0.1"))
	if w := serve(t, mw, fromIP("10.0.0.1")); w.Code != 429 {
		t.Errorf("3rd attempt on same alias from same IP: %d", w.Code)
	}
	// IP lain tidak terpengaruh
	if w := serve(t, mw, fromIP("10.0.0.2")); w.Code != 200 {
		t.Errorf("other IP blocked: %d", w.Code)
	}
	// Per IP: IP B menyebar ke alias lain, batas total 3
	serve(t, mw, fromIP("10.0.0.2"), path("/other/verify"))
	serve(t, mw, fromIP("10.0.0.2"), path("/third/verify"))
	if w := serve(t, mw, fromIP("10.0.0.2"), path("/fourth/verify")); w.Code != 429 {
		t.Errorf("4th attempt across aliases from one IP must hit the per-IP limit: %d", w.Code)
	}
}

func TestVerifyPasswordRules_TruncatesLongAlias(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	c.Request.RemoteAddr = "10.0.0.1:1"
	c.Params = gin.Params{{Key: "alias", Value: strings.Repeat("x", 500)}}

	rules := VerifyPasswordRules(5, 20, time.Minute)(c)
	if len(rules) != 2 || len(rules[0].Key) > len("verify:ip:10.0.0.1:alias:")+maxAliasKeyLen {
		t.Errorf("alias in redis key must be truncated: %q", rules[0].Key)
	}
	if rules[1].Key != "verify:ip:10.0.0.1" {
		t.Errorf("per-IP key = %q", rules[1].Key)
	}
}
