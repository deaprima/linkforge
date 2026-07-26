package main

import (
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/deaprima/linkforge/services/auth/internal/config"
	grpcDelivery "github.com/deaprima/linkforge/services/auth/internal/delivery/grpc"
	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/deaprima/linkforge/services/auth/internal/repository"
	"github.com/deaprima/linkforge/services/auth/internal/service"
	"github.com/deaprima/linkforge/services/shared/database"
	pb "github.com/deaprima/linkforge/services/shared/proto/auth"
	"github.com/joho/godotenv"
	"google.golang.org/grpc" 
)

func main(){

	err := godotenv.Load("../.env")
	if err != nil {
		log.Println("Warning: .env file not found, relying on system environment variables")
	}

	log.Println("Starting Auth Service..")

	cfg := config.LoadConfig()

	db, err := database.ConnectPostgres(cfg.DatabaseDSN, true)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	log.Printf("Running AutoMigration...")
	err = db.AutoMigrate(
		&entity.User{},
		&entity.OAuthAccount{},
		&entity.RefreshToken{},
		&entity.ApiKey{},
	)
	if err != nil {
		log.Fatalf("Database migration failed: %v", err)
	}
	log.Println("Database migration completed successfully!!!")

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewTokenRepository(db)
	keyRepo := repository.NewApiKeyRepository(db)
	oauthRepo := repository.NewOAuthRepository(db)

	tokenManager := service.NewTokenManager(cfg.JWTSecret, cfg.JWTAccessDur)
	authService := service.NewAuthService(userRepo, tokenRepo, keyRepo, oauthRepo, cfg.GoogleClientID, tokenManager, cfg.JWTAccessDur)

	authHandler := grpcDelivery.NewAuthHandler(authService)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", cfg.GRPCPort, err)
	}

	grpcServer := grpc.NewServer()

	pb.RegisterAuthServiceServer(grpcServer, authHandler)

	go func() {
		log.Printf("gRPC server is running on port %s...", cfg.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	log.Printf("Shutting down gRPC server gracefully...")
	grpcServer.GracefulStop()

	log.Printf("Auth Service stoped.")
}