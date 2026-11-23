package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/deaprima/linkforge/services/auth/internal/config"
	grpcDelivery "github.com/deaprima/linkforge/services/auth/internal/delivery/grpc"
	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/deaprima/linkforge/services/auth/internal/mailer"
	"github.com/deaprima/linkforge/services/auth/internal/repository"
	"github.com/deaprima/linkforge/services/auth/internal/service"
	"github.com/deaprima/linkforge/services/shared/database"
	"github.com/deaprima/linkforge/services/shared/logger"
	"github.com/deaprima/linkforge/services/shared/metrics"
	"github.com/deaprima/linkforge/services/shared/ops"
	pb"github.com/deaprima/linkforge/services/shared/proto/auth"
	"github.com/joho/godotenv"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main(){

	envErr := godotenv.Load("../.env")
	logger.Setup("auth") // setelah .env dimuat supaya LOG_LEVEL / LOG_FORMAT terbaca
	if envErr != nil {
		slog.Warn(".env file not found, relying on system environment variables")
	}

	slog.Info("Starting Auth Service..")

	cfg := config.LoadConfig()

	db, err := database.ConnectPostgres(cfg.DatabaseDSN, database.DebugFromEnv())
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}

	log.Printf("Running AutoMigration...")
	err = db.AutoMigrate(
		&entity.User{},
		&entity.OAuthAccount{},
		&entity.RefreshToken{},
		&entity.ApiKey{},
		&entity.PasswordResetToken{},
	)
	if err != nil {
		logger.Fatalf("Database migration failed: %v", err)
	}
	log.Println("Database migration completed successfully!!!")

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewTokenRepository(db)
	keyRepo := repository.NewApiKeyRepository(db)
	oauthRepo := repository.NewOAuthRepository(db)
	resetRepo := repository.NewPasswordResetRepository(db)

	tokenManager := service.NewTokenManager(cfg.JWTSecret, cfg.JWTAccessDur)
	authService := service.NewAuthService(userRepo, tokenRepo, keyRepo, oauthRepo, cfg.GoogleClientID, tokenManager, cfg.JWTRefreshDur,
		service.PasswordResetConfig{
			Repo: resetRepo,
			Mailer: mailer.New(mailer.SMTPConfig{
				Host: cfg.SMTPHost, Port: cfg.SMTPPort, User: cfg.SMTPUser, Pass: cfg.SMTPPass, From: cfg.SMTPFrom,
			}),
			FrontendURL: cfg.FrontendURL,
			TTL:         cfg.ResetTokenTTL,
		})

	authHandler := grpcDelivery.NewAuthHandler(authService)

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Fatalf("Failed to listen on port %s: %v", cfg.GRPCPort, err)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(metrics.UnaryServerInterceptor()),
		grpc.ChainStreamInterceptor(metrics.StreamServerInterceptor()),
	)
	stopOps := ops.Serve(ops.AddrFromEnv("AUTH_METRICS_ADDR", ":9101"))

	pb.RegisterAuthServiceServer(grpcServer, authHandler)

	// Standard gRPC health check, dipakai Gateway untuk /readyz
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	go func() {
		log.Printf("gRPC server is running on port %s...", cfg.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	<-stop

	log.Printf("Shutting down gRPC server gracefully...")
	healthServer.Shutdown()
	grpcServer.GracefulStop()
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	_ = stopOps(shutdownCtx)

	log.Printf("Auth Service stoped.")
}