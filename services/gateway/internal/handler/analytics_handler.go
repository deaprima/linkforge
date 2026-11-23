package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/deaprima/linkforge/services/gateway/internal/middleware"
	"github.com/deaprima/linkforge/services/gateway/internal/response"
	analyticspb "github.com/deaprima/linkforge/services/shared/proto/analytics"
	authpb "github.com/deaprima/linkforge/services/shared/proto/auth"
	linkpb "github.com/deaprima/linkforge/services/shared/proto/link"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
)

// Interval keep-alive WebSocket; var supaya bisa dipercepat di test.
var (
	wsPingInterval = 30 * time.Second
	wsPongWait     = 60 * time.Second
	wsWriteWait    = 10 * time.Second
)

// AnalyticsService adalah subset AnalyticsClient yang dibutuhkan AnalyticsHandler.
type AnalyticsService interface {
	GetLinkAnalytics(ctx context.Context, req *analyticspb.GetLinkAnalyticsRequest) (*analyticspb.GetLinkAnalyticsResponse, error)
	GetSummary(ctx context.Context, req *analyticspb.GetSummaryRequest) (*analyticspb.GetSummaryResponse, error)
	SubscribeRealtime(ctx context.Context, req *analyticspb.SubscribeRealtimeRequest) (grpc.ServerStreamingClient[analyticspb.ClickEvent], error)
}

// TokenValidator dipakai endpoint WebSocket, yang memvalidasi token sendiri (bukan lewat middleware)
// karena token bisa datang dari query param.
type TokenValidator interface {
	ValidateToken(ctx context.Context, req *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error)
}

type AnalyticsHandler struct {
	analytics      AnalyticsService
	links          LinkService
	auth           TokenValidator
	allowedOrigins map[string]bool
	upgrader       websocket.Upgrader
}

func NewAnalyticsHandler(analytics AnalyticsService, links LinkService, auth TokenValidator, allowedOrigins []string) *AnalyticsHandler {
	origins := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		origins[o] = true
	}
	return &AnalyticsHandler{
		analytics:      analytics,
		links:          links,
		auth:           auth,
		allowedOrigins: origins,
		// Origin sudah dicek sebelum upgrade (supaya bisa membalas 403 dengan envelope)
		upgrader: websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }},
	}
}

type countryCountDTO struct {
	Country string `json:"country"`
	Count   int64  `json:"count"`
}

type browserCountDTO struct {
	Browser string `json:"browser"`
	Count   int64  `json:"count"`
}

type deviceCountDTO struct {
	Device string `json:"device"`
	Count  int64  `json:"count"`
}

type dailyClicksDTO struct {
	Date   string `json:"date"`
	Clicks int64  `json:"clicks"`
}

type linkAnalyticsDTO struct {
	LinkID       string            `json:"link_id"`
	TotalClicks  int64             `json:"total_clicks"`
	UniqueClicks int64             `json:"unique_clicks"`
	TopCountries []countryCountDTO `json:"top_countries"`
	TopBrowsers  []browserCountDTO `json:"top_browsers"`
	TopDevices   []deviceCountDTO  `json:"top_devices"`
	TimeSeries   []dailyClicksDTO  `json:"time_series"`
}

type topLinkDTO struct {
	LinkID string `json:"link_id"`
	Alias  string `json:"alias"`
	Clicks int64  `json:"clicks"`
}

type summaryDTO struct {
	TotalLinks  int64        `json:"total_links"`
	TotalClicks int64        `json:"total_clicks"`
	TopLinks    []topLinkDTO `json:"top_links"`
}

type clickEventDTO struct {
	LinkID    string `json:"link_id"`
	Alias     string `json:"alias"`
	Country   string `json:"country"`
	Browser   string `json:"browser"`
	Device    string `json:"device"`
	Timestamp string `json:"timestamp"`
}

type realtimeMessage struct {
	Event string        `json:"event"`
	Data  clickEventDTO `json:"data"`
}

// mapSlice selalu mengembalikan slice non-nil supaya JSON berisi [] bukan null.
func mapSlice[S, D any](src []S, fn func(S) D) []D {
	out := make([]D, 0, len(src))
	for _, s := range src {
		out = append(out, fn(s))
	}
	return out
}

// GET /api/v1/analytics/links/:id?from=&to=
func (h *AnalyticsHandler) LinkStats(c *gin.Context) {
	ctx, linkID, userID := rpcContext(c), c.Param("id"), middleware.UserID(c)

	// Analytics tidak tahu pemilik link — cek ownership ke Link Service dulu (404 kalau bukan miliknya)
	if _, err := h.links.GetLink(ctx, &linkpb.GetLinkRequest{LinkId: linkID, UserId: userID}); err != nil {
		linkError(c, err)
		return
	}

	res, err := h.analytics.GetLinkAnalytics(ctx, &analyticspb.GetLinkAnalyticsRequest{
		LinkId: linkID,
		UserId: userID,
		From:   c.Query("from"),
		To:     c.Query("to"),
	})
	if err != nil {
		response.GRPCError(c, err, nil)
		return
	}

	response.OK(c, http.StatusOK, linkAnalyticsDTO{
		LinkID:       res.GetLinkId(),
		TotalClicks:  res.GetTotalClicks(),
		UniqueClicks: res.GetUniqueClicks(),
		TopCountries: mapSlice(res.GetTopCountries(), func(x *analyticspb.CountryCount) countryCountDTO {
			return countryCountDTO{Country: x.GetCountry(), Count: x.GetCount()}
		}),
		TopBrowsers: mapSlice(res.GetTopBrowsers(), func(x *analyticspb.BrowserCount) browserCountDTO {
			return browserCountDTO{Browser: x.GetBrowser(), Count: x.GetCount()}
		}),
		TopDevices: mapSlice(res.GetTopDevices(), func(x *analyticspb.DeviceCount) deviceCountDTO {
			return deviceCountDTO{Device: x.GetDevice(), Count: x.GetCount()}
		}),
		TimeSeries: mapSlice(res.GetTimeSeries(), func(x *analyticspb.DailyClickCount) dailyClicksDTO {
			return dailyClicksDTO{Date: x.GetDate(), Clicks: x.GetClicks()}
		}),
	})
}

// GET /api/v1/analytics/summary — API composition: daftar link dari Link Service, angka dari Analytics
func (h *AnalyticsHandler) Summary(c *gin.Context) {
	ctx, userID := rpcContext(c), middleware.UserID(c)

	ids, err := h.links.GetUserLinkIDs(ctx, &linkpb.GetUserLinkIDsRequest{UserId: userID})
	if err != nil {
		linkError(c, err)
		return
	}

	res, err := h.analytics.GetSummary(ctx, &analyticspb.GetSummaryRequest{UserId: userID, LinkIds: ids.GetLinkIds()})
	if err != nil {
		response.GRPCError(c, err, nil)
		return
	}

	response.OK(c, http.StatusOK, summaryDTO{
		TotalLinks:  res.GetTotalLinks(),
		TotalClicks: res.GetTotalClicks(),
		TopLinks: mapSlice(res.GetTopLinks(), func(x *analyticspb.TopLink) topLinkDTO {
			return topLinkDTO{LinkID: x.GetLinkId(), Alias: x.GetAlias(), Clicks: x.GetClicks()}
		}),
	})
}

// realtimeToken: query ?token= (browser WebSocket tidak bisa set header), lalu Bearer header / cookie.
func realtimeToken(c *gin.Context) string {
	if token := c.Query("token"); token != "" {
		return token
	}
	if token, ok := middleware.BearerToken(c); ok {
		return token
	}
	return ""
}

// originAllowed: client non-browser tidak mengirim Origin; browser wajib dari origin yang diizinkan.
// Tanpa cek ini, situs lain bisa membuka WebSocket memakai cookie korban (cross-site WebSocket hijacking).
func (h *AnalyticsHandler) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == "" || h.allowedOrigins[origin]
}

// WS /api/v1/analytics/realtime
func (h *AnalyticsHandler) Realtime(c *gin.Context) {
	token := realtimeToken(c)
	if token == "" {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "Authentication required.")
		return
	}
	auth, err := h.auth.ValidateToken(c.Request.Context(), &authpb.ValidateTokenRequest{AccessToken: token})
	if err != nil {
		response.GRPCError(c, err, nil)
		return
	}
	if !auth.GetIsValid() {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "Invalid or expired access token.")
		return
	}
	if !h.originAllowed(c.Request) {
		response.Error(c, http.StatusForbidden, response.CodeForbidden, "Origin not allowed.")
		return
	}

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	stream, err := h.analytics.SubscribeRealtime(ctx, &analyticspb.SubscribeRealtimeRequest{UserId: auth.GetUserId()})
	if err != nil {
		response.GRPCError(c, err, nil)
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return // upgrader sudah menulis respons error HTTP
	}
	defer conn.Close()

	// Read pump: memproses pong & mendeteksi client menutup koneksi
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsPongWait))
	})
	go func() {
		defer cancel()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	// Stream pump: gRPC Recv() memblokir, jadi dipindah ke goroutine sendiri
	events := make(chan *analyticspb.ClickEvent)
	go func() {
		defer close(events)
		for {
			ev, err := stream.Recv()
			if err != nil {
				return
			}
			select {
			case events <- ev:
			case <-ctx.Done():
				return
			}
		}
	}()

	ping := time.NewTicker(wsPingInterval)
	defer ping.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-events:
			if !ok {
				// Stream dari Analytics putus (service restart/mati) — beri tahu client supaya reconnect
				_ = conn.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseTryAgainLater, "analytics stream unavailable"),
					time.Now().Add(wsWriteWait))
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			err := conn.WriteJSON(realtimeMessage{Event: "link.clicked", Data: clickEventDTO{
				LinkID:    ev.GetLinkId(),
				Alias:     ev.GetAlias(),
				Country:   ev.GetCountry(),
				Browser:   ev.GetBrowser(),
				Device:    ev.GetDevice(),
				Timestamp: ev.GetTimestamp(),
			}})
			if err != nil {
				return
			}
		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsWriteWait)); err != nil {
				return
			}
		}
	}
}
