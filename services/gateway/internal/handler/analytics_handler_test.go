package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/deaprima/linkforge/services/gateway/internal/handler"
	"github.com/deaprima/linkforge/services/shared/breaker"
	analyticspb "github.com/deaprima/linkforge/services/shared/proto/analytics"
	linkpb "github.com/deaprima/linkforge/services/shared/proto/link"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func ownLinkOnly(env *testEnv) {
	env.link.getLink = func(_ context.Context, r *linkpb.GetLinkRequest) (*linkpb.GetLinkResponse, error) {
		if r.LinkId != "link-1" || r.UserId != "user-1" {
			return nil, status.Error(codes.NotFound, "link not found")
		}
		return &linkpb.GetLinkResponse{Link: testLink("a")}, nil
	}
}

func TestLinkStats(t *testing.T) {
	env := newEnv()
	ownLinkOnly(env)
	var got *analyticspb.GetLinkAnalyticsRequest
	env.analytics.getLinkAnalytics = func(_ context.Context, r *analyticspb.GetLinkAnalyticsRequest) (*analyticspb.GetLinkAnalyticsResponse, error) {
		got = r
		return &analyticspb.GetLinkAnalyticsResponse{
			LinkId: "link-1", TotalClicks: 254, UniqueClicks: 180,
			TopCountries: []*analyticspb.CountryCount{{Country: "ID", Count: 120}},
			TopBrowsers:  []*analyticspb.BrowserCount{{Browser: "Chrome", Count: 150}},
			TopDevices:   []*analyticspb.DeviceCount{{Device: "mobile", Count: 180}},
			TimeSeries:   []*analyticspb.DailyClickCount{{Date: "2026-07-01", Clicks: 30}},
		}, nil
	}

	w := env.do(t, "GET", "/api/v1/analytics/links/link-1?from=2026-07-01&to=2026-07-31", "", apiKey)
	want := `{"success":true,"data":{"link_id":"link-1","total_clicks":254,"unique_clicks":180,` +
		`"top_countries":[{"country":"ID","count":120}],"top_browsers":[{"browser":"Chrome","count":150}],` +
		`"top_devices":[{"device":"mobile","count":180}],"time_series":[{"date":"2026-07-01","clicks":30}]}}`
	if w.Code != 200 || w.Body.String() != want {
		t.Errorf("got %d %s", w.Code, w.Body.String())
	}
	if got.From != "2026-07-01" || got.To != "2026-07-31" || got.UserId != "user-1" {
		t.Errorf("rpc request = %+v", got)
	}
}

func TestLinkStats_EmptyListsAreArrays(t *testing.T) {
	env := newEnv()
	ownLinkOnly(env)
	env.analytics.getLinkAnalytics = func(context.Context, *analyticspb.GetLinkAnalyticsRequest) (*analyticspb.GetLinkAnalyticsResponse, error) {
		return &analyticspb.GetLinkAnalyticsResponse{LinkId: "link-1"}, nil
	}
	body := env.do(t, "GET", "/api/v1/analytics/links/link-1", "", bearer).Body.String()
	for _, field := range []string{"top_countries", "top_browsers", "top_devices", "time_series"} {
		if !strings.Contains(body, `"`+field+`":[]`) {
			t.Errorf("%s must be [] not null: %s", field, body)
		}
	}
}

func TestLinkStats_Errors(t *testing.T) {
	t.Run("not the owner never reaches analytics", func(t *testing.T) {
		env := newEnv()
		ownLinkOnly(env)
		env.analytics.getLinkAnalytics = func(context.Context, *analyticspb.GetLinkAnalyticsRequest) (*analyticspb.GetLinkAnalyticsResponse, error) {
			t.Fatal("analytics must not be queried for a link the user does not own")
			return nil, nil
		}
		assertError(t, env.do(t, "GET", "/api/v1/analytics/links/someone-elses", "", bearer), 404, "LINK_NOT_FOUND")
	})
	t.Run("requires auth", func(t *testing.T) {
		assertError(t, newEnv().do(t, "GET", "/api/v1/analytics/links/link-1", ""), 401, "UNAUTHORIZED")
	})

	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{"invalid date range", status.Error(codes.InvalidArgument, "invalid argument: from must not be after to"), 400, "VALIDATION_ERROR"},
		{"analytics down", status.Error(codes.Unavailable, "down"), 503, "SERVICE_UNAVAILABLE"},
		{"breaker open", breaker.ErrOpen, 503, "SERVICE_UNAVAILABLE"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newEnv()
			ownLinkOnly(env)
			env.analytics.getLinkAnalytics = func(context.Context, *analyticspb.GetLinkAnalyticsRequest) (*analyticspb.GetLinkAnalyticsResponse, error) {
				return nil, tt.err
			}
			assertError(t, env.do(t, "GET", "/api/v1/analytics/links/link-1", "", bearer), tt.wantStatus, tt.wantCode)
		})
	}
}

func TestSummary(t *testing.T) {
	env := newEnv()
	env.link.getUserLinkIDs = func(_ context.Context, r *linkpb.GetUserLinkIDsRequest) (*linkpb.GetUserLinkIDsResponse, error) {
		if r.UserId != "user-1" {
			t.Errorf("user id = %q", r.UserId)
		}
		return &linkpb.GetUserLinkIDsResponse{LinkIds: []string{"link-1", "link-2"}}, nil
	}
	var gotIDs []string
	env.analytics.getSummary = func(_ context.Context, r *analyticspb.GetSummaryRequest) (*analyticspb.GetSummaryResponse, error) {
		gotIDs = r.LinkIds
		return &analyticspb.GetSummaryResponse{TotalLinks: 2, TotalClicks: 1280,
			TopLinks: []*analyticspb.TopLink{{LinkId: "link-1", Alias: "my-alias", Clicks: 254}}}, nil
	}

	w := env.do(t, "GET", "/api/v1/analytics/summary", "", bearer)
	want := `{"success":true,"data":{"total_links":2,"total_clicks":1280,"top_links":[{"link_id":"link-1","alias":"my-alias","clicks":254}]}}`
	if w.Body.String() != want {
		t.Errorf("got %s", w.Body.String())
	}
	if strings.Join(gotIDs, ",") != "link-1,link-2" {
		t.Errorf("analytics got link ids %v (must come from Link Service)", gotIDs)
	}
}

func TestSummary_Errors(t *testing.T) {
	env := newEnv()
	env.link.getUserLinkIDs = func(context.Context, *linkpb.GetUserLinkIDsRequest) (*linkpb.GetUserLinkIDsResponse, error) {
		return nil, status.Error(codes.Unavailable, "link down")
	}
	assertError(t, env.do(t, "GET", "/api/v1/analytics/summary", "", bearer), 503, "SERVICE_UNAVAILABLE")

	env.link.getUserLinkIDs = func(context.Context, *linkpb.GetUserLinkIDsRequest) (*linkpb.GetUserLinkIDsResponse, error) {
		return &linkpb.GetUserLinkIDsResponse{}, nil
	}
	env.analytics.getSummary = func(context.Context, *analyticspb.GetSummaryRequest) (*analyticspb.GetSummaryResponse, error) {
		return nil, status.Error(codes.Internal, "db down")
	}
	assertError(t, env.do(t, "GET", "/api/v1/analytics/summary", "", bearer), 500, "INTERNAL_ERROR")

	env.analytics.getSummary = func(context.Context, *analyticspb.GetSummaryRequest) (*analyticspb.GetSummaryResponse, error) {
		return &analyticspb.GetSummaryResponse{}, nil
	}
	if body := env.do(t, "GET", "/api/v1/analytics/summary", "", bearer).Body.String(); !strings.Contains(body, `"top_links":[]`) {
		t.Errorf("empty top_links must be []: %s", body)
	}
}

// Realtime WebSocket

// realtimeEnv menyiapkan server HTTP sungguhan + fake stream yang dikendalikan test.
type realtimeEnv struct {
	*testEnv
	server *httptest.Server
	mu     sync.Mutex
	req    *analyticspb.SubscribeRealtimeRequest
	stream *fakeClickStream
	opened chan struct{}
}

func newRealtimeEnv(t *testing.T) *realtimeEnv {
	re := &realtimeEnv{testEnv: newEnv(), opened: make(chan struct{}, 1)}
	re.analytics.subscribeRealtime = func(ctx context.Context, r *analyticspb.SubscribeRealtimeRequest) (grpc.ServerStreamingClient[analyticspb.ClickEvent], error) {
		re.mu.Lock()
		defer re.mu.Unlock()
		re.req = r
		re.stream = &fakeClickStream{ctx: ctx, events: make(chan *analyticspb.ClickEvent, 10)}
		re.opened <- struct{}{}
		return re.stream, nil
	}
	re.server = httptest.NewServer(re.engine(t))
	t.Cleanup(re.server.Close)
	return re
}

func (re *realtimeEnv) dial(t *testing.T, query string, header http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	url := "ws" + strings.TrimPrefix(re.server.URL, "http") + "/api/v1/analytics/realtime" + query
	conn, res, err := websocket.DefaultDialer.Dial(url, header)
	if conn != nil {
		t.Cleanup(func() { conn.Close() })
	}
	return conn, res, err
}

func (re *realtimeEnv) waitOpened(t *testing.T) *fakeClickStream {
	t.Helper()
	select {
	case <-re.opened:
	case <-time.After(2 * time.Second):
		t.Fatal("analytics stream was not opened")
	}
	re.mu.Lock()
	defer re.mu.Unlock()
	return re.stream
}

func TestRealtime_StreamsClicks(t *testing.T) {
	re := newRealtimeEnv(t)
	conn, _, err := re.dial(t, "?token=valid-token", http.Header{"Origin": {frontendURL}})
	if err != nil {
		t.Fatal(err)
	}
	stream := re.waitOpened(t)
	if re.req.UserId != "user-1" {
		t.Errorf("subscribed as %q, want user-1 (from validated token)", re.req.UserId)
	}

	stream.events <- &analyticspb.ClickEvent{LinkId: "link-1", Alias: "promo", Country: "ID", Browser: "Chrome", Device: "mobile", Timestamp: "2026-07-08T07:15:00Z"}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatal(err)
	}
	want := `{"event":"link.clicked","data":{"link_id":"link-1","alias":"promo","country":"ID","browser":"Chrome","device":"mobile","timestamp":"2026-07-08T07:15:00Z"}}`
	if strings.TrimSpace(string(msg)) != want {
		t.Errorf("message = %s", msg)
	}

	// Client menutup koneksi → stream ke Analytics ikut dibatalkan (tidak bocor)
	conn.Close()
	select {
	case <-stream.ctx.Done():
	case <-time.After(2 * time.Second):
		t.Error("analytics stream not cancelled after client disconnect")
	}
}

func TestRealtime_ClosesWhenAnalyticsStreamEnds(t *testing.T) {
	re := newRealtimeEnv(t)
	conn, _, err := re.dial(t, "", http.Header{"Authorization": {"Bearer valid-token"}})
	if err != nil {
		t.Fatal(err)
	}
	close(re.waitOpened(t).events)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = conn.ReadMessage()
	if !websocket.IsCloseError(err, websocket.CloseTryAgainLater) {
		t.Errorf("want close 1013 (try again later) so the client reconnects, got %v", err)
	}
}

func TestRealtime_SendsPings(t *testing.T) {
	defer handler.SetWSTimings(20*time.Millisecond, time.Second)()
	re := newRealtimeEnv(t)
	conn, _, err := re.dial(t, "", http.Header{"Cookie": {"access_token=valid-token"}})
	if err != nil {
		t.Fatal(err)
	}
	re.waitOpened(t)

	pinged := make(chan struct{}, 1)
	conn.SetPingHandler(func(string) error {
		select {
		case pinged <- struct{}{}:
		default:
		}
		return nil
	})
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
	select {
	case <-pinged:
	case <-time.After(2 * time.Second):
		t.Error("server did not send keep-alive ping")
	}
}

func TestRealtime_HandshakeRejections(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		header     http.Header
		wantStatus int
		wantCode   string
	}{
		{"no token", "", nil, 401, "UNAUTHORIZED"},
		{"invalid token", "?token=nope", nil, 401, "UNAUTHORIZED"},
		{"api key is not accepted", "", http.Header{"X-Api-Key": {"lf_sk_valid"}}, 401, "UNAUTHORIZED"},
		{"cross-site origin", "?token=valid-token", http.Header{"Origin": {"https://evil.test"}}, 403, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			re := newRealtimeEnv(t)
			_, res, err := re.dial(t, tt.query, tt.header)
			if err == nil {
				t.Fatal("handshake must fail")
			}
			if res == nil || res.StatusCode != tt.wantStatus {
				t.Fatalf("response = %+v", res)
			}
			if tt.wantCode != "" {
				var body struct {
					Error struct{ Code string } `json:"error"`
				}
				_ = json.NewDecoder(res.Body).Decode(&body)
				if body.Error.Code != tt.wantCode {
					t.Errorf("code = %q, want %q", body.Error.Code, tt.wantCode)
				}
			}
			select {
			case <-re.opened:
				t.Error("analytics stream must not be opened for a rejected handshake")
			default:
			}
		})
	}
}

func TestRealtime_OriginCheckWithoutCORSMiddleware(t *testing.T) {
	// Lapisan kedua: handler sendiri menolak origin asing, tidak hanya bergantung pada middleware CORS
	env := newEnv()
	h := handler.NewAnalyticsHandler(env.analytics, env.link, env.auth, []string{frontendURL})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/ws?token=valid-token", nil)
	c.Request.Header.Set("Origin", "https://evil.test")
	h.Realtime(c)
	assertError(t, w, 403, "FORBIDDEN")
}

func TestRealtime_SubscribeFailure(t *testing.T) {
	env := newEnv()
	env.analytics.subscribeRealtime = func(context.Context, *analyticspb.SubscribeRealtimeRequest) (grpc.ServerStreamingClient[analyticspb.ClickEvent], error) {
		return nil, status.Error(codes.Unavailable, "analytics down")
	}
	assertError(t, env.do(t, "GET", "/api/v1/analytics/realtime?token=valid-token", ""), 503, "SERVICE_UNAVAILABLE")
}
