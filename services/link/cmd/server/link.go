package main

import (
	"log"

	"github.com/deaprima/linkforge/services/link/internal/entity"
	"github.com/deaprima/linkforge/services/shared/database"
)

func main() {
	dsn := "postgres://linkforge:supersecretpassword@localhost:5432/linkforge?sslmode=disable&search_path=link"
	
	db, err := database.ConnectPostgres(dsn, true)
	if err != nil {
		log.Fatalf("Fatal error connecting to database: %v", err)
	}

	log.Println("Running AutoMigration for Link Service...")
	err = db.AutoMigrate(
		&entity.Link{},
		&entity.BlockedDomain{},
	)
	if err != nil {
		log.Fatalf("Fatal error running migrations: %v", err)
	}

	log.Println("Link Service migration completed successfully!")
}
