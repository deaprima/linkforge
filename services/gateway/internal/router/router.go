package router

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/deaprima/linkforge/services/gateway/internal/handler"
	"github.com/deaprima/linkforge/services/gateway/internal/middleware"
	"github.com/deaprima/linkforge/services/gateway/internal/response"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"
)

type Handlers struct {
	Auth      *handler.AuthHandler
	Link      *handler.LinkHandler
	Redirect  *handler.RedirectHandler
	Health    *handler.HealthHandler
	Analytics *handler.AnalyticsHandler
	Validator middleware.CredentialValidator

	// VerifyRateLimit wajib diisi: POST /:alias/verify tanpa brute-force protection tidak boleh jalan
	VerifyRateLimit gin.HandlerFunc
}

type Options struct {
	CORSOrigins    []string
	TrustedProxies []string
}

func New(h Handlers, opts Options) (*gin.Engine, error) {
	if h.VerifyRateLimit == nil {
		return nil, errors.New("router: VerifyRateLimit middleware is required")
	}
	registerJSONTagNames()

	r := gin.New()
	r.Use(gin.LoggerWithFormatter(logFormatter), gin.Recovery())
	if err := r.SetTrustedProxies(opts.TrustedProxies); err != nil {
		return nil, err
	}

	r.Use(cors.New(cors.Config{
		AllowOrigins:     opts.CORSOrigins,
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-API-Key"},
		ExposeHeaders:    []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "Retry-After"},
		AllowCredentials: true, // dibutuhkan untuk cookie hasil OAuth
		MaxAge:           12 * time.Hour,
	}))

	// Ops endpoints (di luar /api/v1)
	r.GET("/healthz", h.Health.Healthz)
	r.GET("/readyz", h.Health.Readyz)

	api := r.Group("/api/v1")

	// Auth
	auth := api.Group("/auth")
	auth.POST("/register", h.Auth.Register)
	auth.POST("/login", h.Auth.Login)
	auth.POST("/refresh", h.Auth.Refresh)
	auth.GET("/oauth/google", h.Auth.GoogleLogin)
	auth.GET("/oauth/google/callback", h.Auth.GoogleCallback)

	// Manajemen akun: Bearer token saja, API key tidak berlaku
	account := auth.Group("", middleware.RequireAuth(h.Validator, false))
	account.POST("/logout", h.Auth.Logout)
	account.GET("/me", h.Auth.Me)
	account.POST("/api-keys", h.Auth.CreateApiKey)
	account.GET("/api-keys", h.Auth.ListApiKeys)
	account.DELETE("/api-keys/:id", h.Auth.DeleteApiKey)

	// Links: Bearer token atau API key
	links := api.Group("/links", middleware.RequireAuth(h.Validator, true))
	links.POST("", h.Link.Create)
	links.GET("", h.Link.List)
	links.GET("/:id", h.Link.Get)
	links.PUT("/:id", h.Link.Update)
	links.DELETE("/:id", h.Link.Delete)
	links.GET("/:id/qr", h.Link.QR)

	// Analytics: Bearer token atau API key; WebSocket memvalidasi token sendiri (bisa via query param)
	analytics := api.Group("/analytics")
	analytics.GET("/realtime", h.Analytics.Realtime)
	stats := analytics.Group("", middleware.RequireAuth(h.Validator, true))
	stats.GET("/summary", h.Analytics.Summary)
	stats.GET("/links/:id", h.Analytics.LinkStats)

	// Redirect publik dari root — route static di atas tetap diprioritaskan oleh Gin
	r.GET("/:alias", h.Redirect.Resolve)
	r.POST("/:alias/verify", h.VerifyRateLimit, h.Redirect.Verify)

	r.NoRoute(func(c *gin.Context) {
		response.Error(c, http.StatusNotFound, response.CodeNotFound, "Route not found.")
	})

	return r, nil
}

// registerJSONTagNames membuat pesan validasi memakai nama field JSON (original_url), bukan nama struct (OriginalURL).
func registerJSONTagNames() {
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			return f.Name
		}
		return name
	})
}

// sensitiveQueryParams tidak boleh muncul di access log (mis. ?token= di WebSocket realtime).
var sensitiveQueryParams = []string{"token", "code", "state"}

// logFormatter = format default gin, tapi query sensitif di-redact.
func logFormatter(p gin.LogFormatterParams) string {
	return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %-7s %#v\n%s",
		p.TimeStamp.Format("2006/01/02 - 15:04:05"),
		p.StatusCode,
		p.Latency,
		p.ClientIP,
		p.Method,
		redactQuery(p.Path),
		p.ErrorMessage,
	)
}

func redactQuery(path string) string {
	base, rawQuery, found := strings.Cut(path, "?")
	if !found {
		return path
	}
	query, err := url.ParseQuery(rawQuery)
	if err != nil {
		return base + "?REDACTED"
	}
	redacted := false
	for _, key := range sensitiveQueryParams {
		if query.Has(key) {
			query.Set(key, "REDACTED")
			redacted = true
		}
	}
	if !redacted {
		return path
	}
	return base + "?" + query.Encode()
}
