package config

import (
	"reflect"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	for k, v := range map[string]string{
		"DB_USER": "u", "DB_PASSWORD": "p", "DB_HOST": "h", "DB_PORT": "5432", "DB_NAME": "n",
		"LINK_PORT": "50052", "KAFKA_BROKERS": "k1:9092,k2:9092",
	} {
		t.Setenv(k, v)
	}

	want := &Config{
		GRPCPort:     "50052",
		DatabaseDSN:  "postgres://u:p@h:5432/n?sslmode=disable&search_path=link",
		KafkaBrokers: []string{"k1:9092", "k2:9092"},
	}
	if got := LoadConfig(); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}
