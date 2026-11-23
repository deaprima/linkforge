package config

import (
	"log"
	"os"
	"strconv"
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

	// Brute-force protection POST /:alias/verify (lihat API-CONTRACT.md)
	VerifyLimitPerAlias	int
	VerifyLimitPerIP	int
	VerifyLimitWindow	time.Duration

	// Lupa password: POST /auth/forgot-password dan /auth/reset-password, per IP
	ForgotLimitPerIP  int
	ForgotLimitWindow time.Duration
	ResetLimitPerIP   int
	ResetLimitWindow  time.Duration

	// Rate limit umum: auth publik & redirect per IP, endpoint terautentikasi per user
	AuthLimitPerIP    int
	AuthLimitWindow   time.Duration
	RefreshLimitPerIP  int
	RefreshLimitWindow time.Duration
	PublicLimitPerIP  int
	PublicLimitWindow time.Duration
	UserLimitPerUser  int
	UserLimitWindow   time.Duration
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

		VerifyLimitPerAlias: getEnvInt("VERIFY_LIMIT_PER_ALIAS", 5),
		VerifyLimitPerIP:    getEnvInt("VERIFY_LIMIT_PER_IP", 20),
		VerifyLimitWindow:   getEnvDuration("VERIFY_LIMIT_WINDOW", 15*time.Minute),

		ForgotLimitPerIP:  getEnvInt("FORGOT_LIMIT_PER_IP", 5),
		ForgotLimitWindow: getEnvDuration("FORGOT_LIMIT_WINDOW", time.Hour),
		ResetLimitPerIP:   getEnvInt("RESET_LIMIT_PER_IP", 20),
		ResetLimitWindow:  getEnvDuration("RESET_LIMIT_WINDOW", 15*time.Minute),

		AuthLimitPerIP:    getEnvInt("AUTH_LIMIT_PER_IP", 20),
		AuthLimitWindow:   getEnvDuration("AUTH_LIMIT_WINDOW", 15*time.Minute),
		RefreshLimitPerIP:  getEnvInt("REFRESH_LIMIT_PER_IP", 120),
		RefreshLimitWindow: getEnvDuration("REFRESH_LIMIT_WINDOW", 15*time.Minute),
		PublicLimitPerIP:  getEnvInt("PUBLIC_LIMIT_PER_IP", 300),
		PublicLimitWindow: getEnvDuration("PUBLIC_LIMIT_WINDOW", time.Minute),
		UserLimitPerUser:  getEnvInt("USER_LIMIT_PER_USER", 120),
		UserLimitWindow:   getEnvDuration("USER_LIMIT_WINDOW", time.Minute),
	}
}

func getEnvInt(key string, def int) int {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	n, err := strconv.Atoi(val)
	if err != nil || n < 1 {
		log.Fatalf("Fatal Config Error: Variable '%s' must be a positive integer, got '%s'", key, val)
	}
	return n
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
