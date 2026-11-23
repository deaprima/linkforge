package config

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type Config struct {
	GRPCPort		string
	DatabaseDSN		string
	KafkaBrokers	[]string
	KafkaGroupID	string
	GeoIPDBPath		string	// opsional: tanpa file GeoLite2, country = "ZZ" (unknown)
}

func LoadConfig() *Config {
	// Merakit DSN secara dinamis khusus untuk Analytics Service (search_path=analytics)
	dbDSN := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&search_path=analytics",
		getEnv("DB_USER"),
		getEnv("DB_PASSWORD"),
		getEnv("DB_HOST"),
		getEnv("DB_PORT"),
		getEnv("DB_NAME"),
	)

	return &Config{
		GRPCPort:     getEnv("ANALYTICS_PORT"),
		DatabaseDSN:  dbDSN,
		KafkaBrokers: strings.Split(getEnv("KAFKA_BROKERS"), ","),
		KafkaGroupID: getEnvDefault("KAFKA_GROUP_ID", "analytics-service"),
		GeoIPDBPath:  os.Getenv("GEOIP_DB_PATH"),
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
