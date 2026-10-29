package database

import (
	"os"
	"testing"
)

func TestConnectPostgres_InvalidDSN(t *testing.T) {
	db, err := ConnectPostgres("this is not a dsn", false)
	if err == nil || db != nil {
		t.Errorf("want error for invalid dsn, got db=%v err=%v", db, err)
	}
}

func TestConnectPostgres_Unreachable(t *testing.T) {
	// Port 1 tidak mungkin ada Postgres; GORM harus mengembalikan error, bukan panic.
	_, err := ConnectPostgres("postgres://u:p@127.0.0.1:1/db?sslmode=disable&connect_timeout=1", false)
	if err == nil {
		t.Error("want error for unreachable database")
	}
}

func TestConnectPostgres_Integration(t *testing.T) {
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		t.Skip("TEST_DSN not set; skipping Postgres integration test")
	}
	for _, debug := range []bool{false, true} {
		db, err := ConnectPostgres(dsn, debug)
		if err != nil {
			t.Fatalf("debug=%v: %v", debug, err)
		}
		sqlDB, _ := db.DB()
		if err := sqlDB.Ping(); err != nil {
			t.Errorf("debug=%v: ping: %v", debug, err)
		}
		if got := sqlDB.Stats().MaxOpenConnections; got != 100 {
			t.Errorf("MaxOpenConnections = %d, want 100", got)
		}
		sqlDB.Close()
	}
}
