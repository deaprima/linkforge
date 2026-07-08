package main

import (
	"log"

	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/deaprima/linkforge/services/shared/database"
)

func main() {
	dsn := "postgres://linkforge:supersecretpassword@localhost:5432/linkforge?sslmode=disable&search_path=auth"
	
	db, err := database.ConnectPostgres(dsn, true)
	if err != nil {
		log.Fatalf("Fatal error connecting to database: %v", err)
	}

	log.Println("Running AutoMigration for Auth Service...")
	err = db.AutoMigrate(
		&entity.User{},
		&entity.OAuthAccount{},
		&entity.RefreshToken{},
		&entity.ApiKey{},
	)
	if err != nil {
		log.Fatalf("Fatal error running migrations: %v", err)
	}

	log.Println("Auth Service migration completed successfully!")
}
