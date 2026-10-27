package config

import (
	"reflect"
	"testing"
	"time"
)

func setRequired(t *testing.T) {
	t.Setenv("GATEWAY_PORT", "8080")
	t.Setenv("AUTH_GRPC_ADDR", "localhost:50051")
	t.Setenv("LINK_GRPC_ADDR", "localhost:50052")
	t.Setenv("ANALYTICS_GRPC_ADDR", "localhost:50053")
	t.Setenv("REDIS_ADDR", "localhost:6379")
}

func TestLoadConfig_Defaults(t *testing.T) {
	setRequired(t)
	for _, key := range []string{"BASE_URL", "FRONTEND_URL", "CORS_ALLOWED_ORIGINS", "TRUSTED_PROXIES", "GRPC_TIMEOUT",
		"JWT_ACCESS_DURATION", "JWT_REFRESH_DURATION", "COOKIE_SECURE", "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET", "GOOGLE_REDIRECT_URL",
		"VERIFY_LIMIT_PER_ALIAS", "VERIFY_LIMIT_PER_IP", "VERIFY_LIMIT_WINDOW"} {
		t.Setenv(key, "")
	}

	cfg := LoadConfig()

	want := &Config{
		HTTPPort:          "8080",
		AuthGRPCAddr:      "localhost:50051",
		LinkGRPCAddr:      "localhost:50052",
		AnalyticsGRPCAddr: "localhost:50053",
		RedisAddr:         "localhost:6379",
		BaseURL:           "http://localhost:8080",
		FrontendURL:       "http://localhost:3000",
		CORSOrigins:       []string{"http://localhost:3000"},
		GRPCTimeout:       5 * time.Second,
		AccessTokenDur:    15 * time.Minute,
		RefreshTokenDur:   7 * 24 * time.Hour,
		CookieSecure:      true,
		GoogleRedirectURL: "http://localhost:8080/api/v1/auth/oauth/google/callback",
		VerifyLimitPerAlias: 5,
		VerifyLimitPerIP:    20,
		VerifyLimitWindow:   15 * time.Minute,
	}
	if !reflect.DeepEqual(cfg, want) {
		t.Errorf("got  %+v\nwant %+v", cfg, want)
	}
}

func TestLoadConfig_Overrides(t *testing.T) {
	setRequired(t)
	t.Setenv("BASE_URL", "https://linkforge.deaprima.my.id/")
	t.Setenv("FRONTEND_URL", "https://app.example.com/")
	t.Setenv("CORS_ALLOWED_ORIGINS", " https://a.com , ,https://b.com ")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1")
	t.Setenv("GRPC_TIMEOUT", "2s")
	t.Setenv("JWT_ACCESS_DURATION", "30m")
	t.Setenv("COOKIE_SECURE", "false")

	cfg := LoadConfig()

	if cfg.BaseURL != "https://linkforge.deaprima.my.id" || cfg.FrontendURL != "https://app.example.com" {
		t.Errorf("trailing slash must be trimmed: base=%q frontend=%q", cfg.BaseURL, cfg.FrontendURL)
	}
	if !reflect.DeepEqual(cfg.CORSOrigins, []string{"https://a.com", "https://b.com"}) {
		t.Errorf("CORS origins = %q", cfg.CORSOrigins)
	}
	if !reflect.DeepEqual(cfg.TrustedProxies, []string{"10.0.0.1"}) {
		t.Errorf("trusted proxies = %q", cfg.TrustedProxies)
	}
	if cfg.GRPCTimeout != 2*time.Second || cfg.AccessTokenDur != 30*time.Minute || cfg.CookieSecure {
		t.Errorf("timeout=%v access=%v secure=%v", cfg.GRPCTimeout, cfg.AccessTokenDur, cfg.CookieSecure)
	}
}
