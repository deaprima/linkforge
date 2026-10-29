// Package testdb menyiapkan Postgres sungguhan untuk integration test repository.
package testdb

import (
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// EnvDSN adalah env yang dibaca Open. ANALYTICS_TEST_DSN tetap diterima sebagai fallback
// supaya perintah lama (make test-integration TEST_DSN=...) tidak rusak.
const EnvDSN = "TEST_DSN"

// Open membuat schema sementara yang unik, menjalankan AutoMigrate untuk models, dan
// mengembalikan koneksi yang search_path-nya diarahkan ke schema itu. Schema di-drop setelah
// test selesai, jadi test bisa jalan paralel dan tidak menyentuh data lain di database.
//
// Test di-skip kalau DSN tidak di-set, mis.:
//
//	TEST_DSN="postgres://user:pass@localhost:5432/linkforge?sslmode=disable" go test ./...
func Open(t *testing.T, prefix string, models ...any) *gorm.DB {
	t.Helper()
	dsn := os.Getenv(EnvDSN)
	if dsn == "" {
		dsn = os.Getenv("ANALYTICS_TEST_DSN")
	}
	if dsn == "" {
		t.Skip(EnvDSN + " not set; skipping Postgres integration test")
	}

	schema := prefix + "_test_" + strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		closeDB(admin)
	})

	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	db, err := gorm.Open(postgres.Open(dsn+sep+"search_path="+schema), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { closeDB(db) })

	if len(models) > 0 {
		if err := db.AutoMigrate(models...); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func closeDB(db *gorm.DB) {
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.Close()
	}
}
