package config

import (
	"log"
	"os"
	"strings"
	"time"
)

type Config struct {
	HTTPPort		string
	AuthGRPCAddr	string
	LinkGRPCAddr	string
	AnalyticsGRPCAddr	string
	RedisAddr		string

	BaseURL			string		// prefix short_url, mis. https://linkforge.deaprima.my.id
	FrontendURL		string		// tujuan redirect setelah OAuth
	CORSOrigins		[]string
	TrustedProxies	[]string	// kosong = tidak percaya X-Forwarded-For dari siapapun
	GRPCTimeout		time.Duration

	// Durasi cookie mengikuti durasi token di Auth Service
	AccessTokenDur	time.Duration
	RefreshTokenDur	time.Duration
	CookieSecure	bool

	// Google OAuth opsional: kalau GOOGLE_CLIENT_ID kosong, endpoint OAuth mengembalikan 503
	GoogleClientID		string
	GoogleClientSecret	string
	GoogleRedirectURL	string
}

func LoadConfig() *Config {
	port := getEnv("GATEWAY_PORT")
	frontendURL := getEnvDefault("FRONTEND_URL", "http://localhost:3000")

	return &Config{
		HTTPPort:     port,
		AuthGRPCAddr: getEnv("AUTH_GRPC_ADDR"),
		AnalyticsGRPCAddr: getEnv("ANALYTICS_GRPC_ADDR"),
		LinkGRPCAddr: getEnv("LINK_GRPC_ADDR"),
		RedisAddr:    getEnv("REDIS_ADDR"),

		BaseURL:        strings.TrimRight(getEnvDefault("BASE_URL", "http://localhost:"+port), "/"),
		FrontendURL:    strings.TrimRight(frontendURL, "/"),
		CORSOrigins:    getEnvList("CORS_ALLOWED_ORIGINS", []string{frontendURL}),
		TrustedProxies: getEnvList("TRUSTED_PROXIES", nil),
		GRPCTimeout:    getEnvDuration("GRPC_TIMEOUT", 5*time.Second),

		AccessTokenDur:  getEnvDuration("JWT_ACCESS_DURATION", 15*time.Minute),
		RefreshTokenDur: getEnvDuration("JWT_REFRESH_DURATION", 7*24*time.Hour),
		CookieSecure:    getEnvDefault("COOKIE_SECURE", "true") != "false",

		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  getEnvDefault("GOOGLE_REDIRECT_URL", "http://localhost:"+port+"/api/v1/auth/oauth/google/callback"),
	}
}

func getEnv(key string) string {
    val := os.Getenv(key)
    if val == "" {
        log.Fatalf("Fatal Config Error: Environment variable '%s' is required but not set!", key)
    }
    return val
}

func getEnvDefault(key, def string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return def
}

func getEnvList(key string, def []string) []string {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	var list []string
	for _, item := range strings.Split(val, ",") {
		if item = strings.TrimSpace(item); item != "" {
			list = append(list, item)
		}
	}
	return list
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		log.Fatalf("Fatal Config Error: Variable '%s' has invalid duration format '%s' (example: '15m', '168h'). Error: %v", key, val, err)
	}
	return d
}
