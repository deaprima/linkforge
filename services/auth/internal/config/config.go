package config

import (
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
}

func LoadConfig() *Config {
	return &Config{
		GRPCPort: getEnv("AUTH_PORT"),
		DatabaseDSN: getEnv("DB_DSN"),
		JWTSecret: getEnv("JWT_SECRET"),
		JWTAccessDur: getEnvDuration("JWT_ACCESS_DURATION"),
		JWTRefreshDur: getEnvDuration("JWT_REFRESH_DURATION"),
	}
}

func getEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Fatal Config Error: Environment variable '%s' is required but not set!", key)
	}
	return val
}

func getEnvDuration(key string) time.Duration {
	val := getEnv(key)

	d, err := time.ParseDuration(val)
	if err != nil {
		log.Fatalf("Fatal Config Error: Variable '%s' has invalid duration format '%s' (example: '15h', '168h'). Error: %v", key, val, err)
	}
	return d
}