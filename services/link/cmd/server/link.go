package main

import (
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"

    "github.com/deaprima/linkforge/services/link/internal/config"
    grpcDelivery "github.com/deaprima/linkforge/services/link/internal/delivery/grpc"
    "github.com/deaprima/linkforge/services/link/internal/entity"
    "github.com/deaprima/linkforge/services/link/internal/repository"
    "github.com/deaprima/linkforge/services/link/internal/service"
    "github.com/deaprima/linkforge/services/shared/database"
    pb "github.com/deaprima/linkforge/services/shared/proto/link"
    "github.com/joho/godotenv"
    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func main() {
    if err := godotenv.Load(); err != nil {
        log.Println("No .env file found, reading from environment")
    }

    cfg := config.LoadConfig()

    // Koneksi database
    db, err := database.ConnectPostgres(cfg.DatabaseDSN, true)
    if err != nil {
        log.Fatalf("Failed to connect to database: %v", err)
    }

    // Auto migration
    log.Println("Running AutoMigration for Link Service...")
    err = db.AutoMigrate(
        &entity.Link{},
        &entity.BlockedDomain{},
    )
    if err != nil {
        log.Fatalf("Database migration failed: %v", err)
    }
    log.Println("Database migration completed successfully!")

    // Dependency injection
    linkRepo   := repository.NewLinkRepository(db)
    domainRepo := repository.NewDomainRepository(db)

    linkService := service.NewLinkService(linkRepo, domainRepo)
    linkHandler := grpcDelivery.NewLinkHandler(linkService)

    // Setup gRPC server
    lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
    if err != nil {
        log.Fatalf("Failed to listen on port %s: %v", cfg.GRPCPort, err)
    }

    grpcServer := grpc.NewServer()
    pb.RegisterLinkServiceServer(grpcServer, linkHandler)

    // Standard gRPC health check, dipakai Gateway untuk /readyz
    healthServer := health.NewServer()
    healthpb.RegisterHealthServer(grpcServer, healthServer)

    go func() {
        log.Printf("Link Service gRPC server running on port %s...", cfg.GRPCPort)
        if err := grpcServer.Serve(lis); err != nil {
            log.Fatalf("Failed to serve gRPC: %v", err)
        }
    }()

    // Graceful shutdown
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop

    log.Println("Shutting down Link Service gracefully...")
    healthServer.Shutdown()
    grpcServer.GracefulStop()
    log.Println("Link Service stopped.")
}
