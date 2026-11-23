package database

import (
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ConnectPostgres membuka koneksi ke PostgreSQL dengan GORM
func ConnectPostgres(dsn string, debug bool) (*gorm.DB, error) {
	config := &gorm.Config{Logger: newLogger(debug, os.Stdout)}

	db, err := gorm.Open(postgres.Open(dsn), config)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	// Setup Connection Pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	log.Println("Successfully connected to PostgreSQL database")
	return db, nil
}

// DebugFromEnv: GORM mencetak setiap SQL beserta nilai parameternya (email, hash password, hash token), jadi
// hanya aktif kalau DB_DEBUG=true secara eksplisit (development). Di produksi dibiarkan mati.
func DebugFromEnv() bool {
	return os.Getenv("DB_DEBUG") == "true"
}

// newLogger: logger bawaan GORM (tanpa konfigurasi) mencatat setiap lookup "record not found" lengkap dengan
// SQL dan NILAInya (email, alias, hash). Itu alur normal (login dengan email tak dikenal, alias yang belum ada)
// dan membocorkan data pribadi ke log. Mode normal: abaikan "record not found" dan jangan sertakan nilai
// parameter (query lambat dan error tetap tercatat). debug = semua SQL dengan nilainya, hanya untuk development.
func newLogger(debug bool, w io.Writer) logger.Interface {
	if debug {
		return logger.New(log.New(w, "", log.LstdFlags), logger.Config{SlowThreshold: 200 * time.Millisecond, LogLevel: logger.Info, Colorful: false})
	}
	return logger.New(log.New(w, "", log.LstdFlags), logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		ParameterizedQueries:      true,
	})
}
