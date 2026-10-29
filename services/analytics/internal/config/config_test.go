package config

import (
	"reflect"
	"testing"
)

func setRequired(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"DB_USER": "u", "DB_PASSWORD": "p", "DB_HOST": "h", "DB_PORT": "5432", "DB_NAME": "n",
		"ANALYTICS_PORT": "50053", "KAFKA_BROKERS": "k1:9092,k2:9092",
	} {
		t.Setenv(k, v)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	setRequired(t)
	t.Setenv("KAFKA_GROUP_ID", "")
	t.Setenv("GEOIP_DB_PATH", "")

	want := &Config{
		GRPCPort:     "50053",
		DatabaseDSN:  "postgres://u:p@h:5432/n?sslmode=disable&search_path=analytics",
		KafkaBrokers: []string{"k1:9092", "k2:9092"},
		KafkaGroupID: "analytics-service",
	}
	if got := LoadConfig(); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestLoadConfig_Overrides(t *testing.T) {
	setRequired(t)
	t.Setenv("KAFKA_GROUP_ID", "custom-group")
	t.Setenv("GEOIP_DB_PATH", "/data/GeoLite2-Country.mmdb")

	got := LoadConfig()
	if got.KafkaGroupID != "custom-group" || got.GeoIPDBPath != "/data/GeoLite2-Country.mmdb" {
		t.Errorf("got %+v", got)
	}
}
