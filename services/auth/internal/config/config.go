package config

import (
	"fmt"
	"os"
	"time"
	"log"
)

type Config struct {
	GRPCPort		string
	DatabaseDSN		string
	JWTSecret 		string
	JWTAccessDur	time.Duration
	JWTRefreshDur	time.Duration
	GoogleClientID	string

	// Lupa password: link di email mengarah ke FrontendURL, dikirim lewat SMTP (kosong = hanya dicetak ke log)
	FrontendURL    string
	ResetTokenTTL  time.Duration
	SMTPHost       string
	SMTPPort       string
	SMTPUser       string
	SMTPPass       string
	SMTPFrom       string
}

func LoadConfig() *Config {

	// Merakit DSN secara dinamis khusus untuk Auth Service (search_path=auth)
	dbDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=auth",
		getEnv("DB_USER"),
		getEnv("DB_PASSWORD"),
		getEnv("DB_HOST"),
		getEnv("DB_PORT"),
		getEnv("DB_NAME"),
	)

	return &Config{
		GRPCPort: getEnv("AUTH_PORT"),
		DatabaseDSN: dbDSN,
		JWTSecret: getEnv("JWT_SECRET"),
		JWTAccessDur: getEnvDuration("JWT_ACCESS_DURATION"),
		JWTRefreshDur: getEnvDuration("JWT_REFRESH_DURATION"),
		GoogleClientID: getEnv("GOOGLE_CLIENT_ID"),

		FrontendURL:   getEnvDefault("FRONTEND_URL", "http://localhost:5173"),
		ResetTokenTTL: getEnvDurationDefault("PASSWORD_RESET_TTL", time.Hour),
		SMTPHost:      os.Getenv("SMTP_HOST"),
		SMTPPort:      getEnvDefault("SMTP_PORT", "587"),
		SMTPUser:      os.Getenv("SMTP_USER"),
		SMTPPass:      os.Getenv("SMTP_PASS"),
		SMTPFrom:      getEnvDefault("SMTP_FROM", "LinkForge <no-reply@linkforge.local>"),
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

func getEnvDurationDefault(key string, def time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		log.Fatalf("Fatal Config Error: Variable '%s' has invalid duration format '%s'. Error: %v", key, val, err)
	}
	return d
}

func getEnvDuration(key string) time.Duration {
	val := getEnv(key)

	d, err := time.ParseDuration(val)
	if err != nil {
		log.Fatalf("Fatal Config Error: Variable '%s' has invalid duration format '%s' (example: '15h', '168h'). Error: %v", key, val, err)
	}
	return d
}