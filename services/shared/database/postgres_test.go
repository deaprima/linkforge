package database

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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

func TestDebugFromEnv(t *testing.T) {
	cases := map[string]bool{"": false, "false": false, "1": false, "TRUE": false, "true": true}
	for in, want := range cases {
		t.Setenv("DB_DEBUG", in)
		if got := DebugFromEnv(); got != want {
			t.Errorf("DB_DEBUG=%q: DebugFromEnv() = %v, want %v", in, got, want)
		}
	}
}

// Lookup yang tidak menemukan baris (alur normal) tidak boleh menulis SQL beserta nilainya ke log.
func TestNewLogger_DoesNotLeakQueryValues(t *testing.T) {
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		t.Skip("TEST_DSN not set; skipping Postgres integration test")
	}
	const secret = "victim@example.com"

	run := func(build func(w io.Writer) logger.Interface) string {
		var buf bytes.Buffer
		db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: build(&buf)})
		if err != nil {
			t.Fatal(err)
		}
		sqlDB, _ := db.DB()
		defer sqlDB.Close()

		var n int
		// Lookup tanpa hasil, persis seperti login dengan email tak dikenal
		_ = db.Table("pg_catalog.pg_class").Where("relname = ?", secret).Take(&n).Error
		return buf.String()
	}

	// Kontrol: konfigurasi logger bawaan GORM memang membocorkan nilainya. Tanpa ini, test di bawah bisa
	// lolos hanya karena tidak ada yang tertangkap.
	control := run(func(w io.Writer) logger.Interface {
		return logger.New(log.New(w, "", 0), logger.Config{LogLevel: logger.Warn})
	})
	if !strings.Contains(control, secret) {
		t.Fatalf("control did not capture the query value; the leak check below would be meaningless:\n%q", control)
	}

	if got := run(func(w io.Writer) logger.Interface { return newLogger(false, w) }); strings.Contains(got, secret) {
		t.Errorf("query values leaked to the log:\n%s", got)
	}
}

func TestNewLogger_DebugModeLogsQueries(t *testing.T) {
	dsn := os.Getenv("TEST_DSN")
	if dsn == "" {
		t.Skip("TEST_DSN not set; skipping Postgres integration test")
	}
	var buf bytes.Buffer
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: newLogger(true, &buf)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	var n int
	_ = db.Raw("SELECT 1").Scan(&n).Error
	if !strings.Contains(buf.String(), "SELECT 1") {
		t.Errorf("debug mode must log queries, got %q", buf.String())
	}
}
