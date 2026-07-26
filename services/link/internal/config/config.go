package config

import (
	"fmt"
	"log"
	"os"
	"time"
)

type Config struct {
	GRPCPort	string
	DatabaseDSN	string
}

func LoadConfig() *Config {
	dbDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=link",
		getEnv("DB_USER"),
		getEnv("DB_PASSWORD"),
		getEnv("DB_HOST"),
		getEnv("DB_PORT"),
		getEnv("DB_NAME"),
	) 
	return &Config{
		GRPCPort: 	getEnv("LINK_PORT"),
		DatabaseDSN: dbDSN,
	}
}

func getEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("Fatal Config Error: Environtment variable '%s' is required but not set!", key)
	} 
	return val
}

func getEnvDuration(key string) time.Duration {
	val := getEnv(key)
	d, err := time.ParseDuration(val)
	if err != nil {
		log.Fatalf("Fatal Config Error: Variable '%s' has invalid duration format '%s'. Error: %v", key, val, err)
	}
	return d
}
