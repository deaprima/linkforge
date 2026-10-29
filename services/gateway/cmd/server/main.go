package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/deaprima/linkforge/services/gateway/internal/client"
	"github.com/deaprima/linkforge/services/gateway/internal/config"
	"github.com/deaprima/linkforge/services/gateway/internal/handler"
	"github.com/deaprima/linkforge/services/gateway/internal/ratelimit"
	"github.com/deaprima/linkforge/services/gateway/internal/router"
	"github.com/deaprima/linkforge/services/shared/logger"
	"github.com/deaprima/linkforge/services/shared/ops"
	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	envErr := godotenv.Load("../.env")
	logger.Setup("gateway") // setelah .env dimuat supaya LOG_LEVEL / LOG_FORMAT terbaca
	if envErr != nil {
		slog.Warn(".env file not found, relying on system environment variables")
	}

	slog.Info("Starting Gateway Service..")

	cfg := config.LoadConfig()

	// gRPC clients (koneksi lazy — service tujuan tidak harus sudah jalan saat startup)
	authClient, err := client.NewAuthClient(cfg.AuthGRPCAddr, cfg.GRPCTimeout)
	if err != nil {
		logger.Fatalf("Failed to create auth client: %v", err)
	}
	defer authClient.Close()

	linkClient, err := client.NewLinkClient(cfg.LinkGRPCAddr, cfg.GRPCTimeout)
	if err != nil {
		logger.Fatalf("Failed to create link client: %v", err)
	}
	defer linkClient.Close()

	analyticsClient, err := client.NewAnalyticsClient(cfg.AnalyticsGRPCAddr, cfg.GRPCTimeout)
	if err != nil {
		logger.Fatalf("Failed to create analytics client: %v", err)
	}
	defer analyticsClient.Close()

	// Redis: state rate limiter (+ readiness)
	rdb := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer rdb.Close()
	limiter := ratelimit.NewRedisLimiter(rdb, "linkforge:rl:")

	// Dependency injection
	cookies := handler.CookieConfig{
		Secure:          cfg.CookieSecure,
		AccessTokenTTL:  cfg.AccessTokenDur,
		RefreshTokenTTL: cfg.RefreshTokenDur,
	}
	google := handler.NewGoogleOAuth(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURL)
	if !google.Enabled() {
		log.Println("Warning: GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET not set, Google OAuth endpoints disabled")
	}

	engine, err := router.New(router.Handlers{
		Auth:     handler.NewAuthHandler(authClient, google, cookies, cfg.FrontendURL),
		Link:     handler.NewLinkHandler(linkClient, cfg.BaseURL),
		Redirect: handler.NewRedirectHandler(linkClient, cfg.FrontendURL),
		Health: handler.NewHealthHandler(map[string]handler.CheckFunc{
			"auth":      authClient.Check,
			"link":      linkClient.Check,
			"analytics": analyticsClient.Check,
			"redis":     func(ctx context.Context) error { return rdb.Ping(ctx).Err() },
		}),
		Analytics: handler.NewAnalyticsHandler(analyticsClient, linkClient, authClient, cfg.CORSOrigins),
		Validator: authClient,
		// Fail-closed: kalau Redis mati, verifikasi password ditolak (brute-force tidak boleh lolos)
		VerifyRateLimit: ratelimit.Middleware(limiter,
			ratelimit.VerifyPasswordRules(cfg.VerifyLimitPerAlias, cfg.VerifyLimitPerIP, cfg.VerifyLimitWindow),
			ratelimit.Options{Message: "Too many password attempts. Try again later."}),
		// Fail-closed juga: tanpa limiter, endpoint ini bisa dipakai membanjiri inbox orang lain
		ForgotRateLimit: ratelimit.Middleware(limiter,
			ratelimit.ForgotPasswordRules(cfg.ForgotLimitPerIP, cfg.ForgotLimitWindow),
			ratelimit.Options{Message: "Too many reset requests. Try again later."}),
		ResetRateLimit: ratelimit.Middleware(limiter,
			ratelimit.ResetPasswordRules(cfg.ResetLimitPerIP, cfg.ResetLimitWindow),
			ratelimit.Options{Message: "Too many attempts. Try again later."}),
		// Fail-closed: login/register tanpa limiter membuka credential stuffing
		AuthRateLimit: ratelimit.Middleware(limiter,
			ratelimit.AuthRules(cfg.AuthLimitPerIP, cfg.AuthLimitWindow),
			ratelimit.Options{Message: "Too many authentication attempts. Try again later."}),
		// Fail-open: redirect & API biasa tetap jalan kalau Redis mati (hanya kehilangan throttling)
		PublicRateLimit: ratelimit.Middleware(limiter,
			ratelimit.PublicRules(cfg.PublicLimitPerIP, cfg.PublicLimitWindow),
			ratelimit.Options{FailOpen: true}),
		UserRateLimit: ratelimit.Middleware(limiter,
			ratelimit.UserRules(cfg.UserLimitPerUser, cfg.UserLimitWindow),
			ratelimit.Options{FailOpen: true}),
	}, router.Options{
		CORSOrigins:    cfg.CORSOrigins,
		TrustedProxies: cfg.TrustedProxies,
	})
	if err != nil {
		logger.Fatalf("Failed to setup router: %v", err)
	}

	srv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// /metrics di listener terpisah dari port publik, jadi tidak pernah terjangkau lewat reverse proxy ke internet
	stopOps := ops.Serve(ops.AddrFromEnv("GATEWAY_METRICS_ADDR", ":9100"))

	go func() {
		log.Printf("Gateway HTTP server running on port %s...", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("Failed to serve HTTP: %v", err)
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down Gateway gracefully...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Gateway forced to shutdown: %v", err)
	}
	_ = stopOps(ctx)
	log.Println("Gateway stopped.")
}
