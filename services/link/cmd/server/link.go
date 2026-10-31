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

    "github.com/deaprima/linkforge/services/link/internal/config"
    grpcDelivery "github.com/deaprima/linkforge/services/link/internal/delivery/grpc"
    "github.com/deaprima/linkforge/services/link/internal/entity"
    "github.com/deaprima/linkforge/services/link/internal/event"
    "github.com/deaprima/linkforge/services/link/internal/repository"
    "github.com/deaprima/linkforge/services/link/internal/service"
    "github.com/deaprima/linkforge/services/shared/database"
    "github.com/deaprima/linkforge/services/shared/kafka"
    pb "github.com/deaprima/linkforge/services/shared/proto/link"
    "github.com/deaprima/linkforge/services/shared/logger"
    "github.com/deaprima/linkforge/services/shared/metrics"
    "github.com/deaprima/linkforge/services/shared/ops"
    "github.com/joho/godotenv"
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
    "google.golang.org/grpc"
    "google.golang.org/grpc/health"
    healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const clickTopicPartitions = 3

func main() {
    // Sama seperti Auth & Gateway: dijalankan dari folder services/, .env ada di root repo
    envErr := godotenv.Load("../.env")
    logger.Setup("link") // setelah .env dimuat supaya LOG_LEVEL / LOG_FORMAT terbaca
    if envErr != nil {
        slog.Warn("No .env file found, reading from environment")
    }

    cfg := config.LoadConfig()

    // Koneksi database
    db, err := database.ConnectPostgres(cfg.DatabaseDSN, database.DebugFromEnv())
    if err != nil {
        logger.Fatalf("Failed to connect to database: %v", err)
    }

    // Auto migration
    log.Println("Running AutoMigration for Link Service...")
    err = db.AutoMigrate(
        &entity.Link{},
        &entity.BlockedDomain{},
    )
    if err != nil {
        logger.Fatalf("Database migration failed: %v", err)
    }
    log.Println("Database migration completed successfully!")

    // Kafka: pastikan topic ada. Tidak fatal — redirect tetap jalan walau Kafka belum siap,
    // event hanya gagal terkirim (lihat KafkaPublisher).
    topicCtx, cancelTopic := context.WithTimeout(context.Background(), 10*time.Second)
    if err := kafka.EnsureTopic(topicCtx, cfg.KafkaBrokers[0], kafka.TopicLinkClicked, clickTopicPartitions); err != nil {
        log.Printf("Warning: could not ensure kafka topic %q: %v", kafka.TopicLinkClicked, err)
    }
    cancelTopic()
    publisher := event.NewKafkaPublisher(event.NewKafkaWriter(cfg.KafkaBrokers, kafka.TopicLinkClicked), 0)

    // Dependency injection
    linkRepo   := repository.NewLinkRepository(db)
    domainRepo := repository.NewDomainRepository(db)

    linkService := service.NewLinkService(linkRepo, domainRepo, publisher)
    linkHandler := grpcDelivery.NewLinkHandler(linkService)

    // Setup gRPC server
    lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
    if err != nil {
        logger.Fatalf("Failed to listen on port %s: %v", cfg.GRPCPort, err)
    }

    grpcServer := grpc.NewServer(
        grpc.ChainUnaryInterceptor(metrics.UnaryServerInterceptor()),
        grpc.ChainStreamInterceptor(metrics.StreamServerInterceptor()),
    )
    pb.RegisterLinkServiceServer(grpcServer, linkHandler)

    // Event klik yang tidak sampai ke Kafka: dibuang (buffer penuh) atau gagal dikirim
    promauto.NewCounterFunc(prometheus.CounterOpts{
        Name: "link_click_events_dropped_total",
        Help: "Event link.clicked dibuang karena buffer publisher penuh.",
    }, func() float64 { return float64(publisher.Dropped()) })
    promauto.NewCounterFunc(prometheus.CounterOpts{
        Name: "link_click_events_failed_total",
        Help: "Event link.clicked yang gagal dikirim ke Kafka.",
    }, func() float64 { return float64(publisher.Failed()) })
    stopOps := ops.Serve(ops.AddrFromEnv("LINK_METRICS_ADDR", ":9102"))

    // Standard gRPC health check, dipakai Gateway untuk /readyz
    healthServer := health.NewServer()
    healthpb.RegisterHealthServer(grpcServer, healthServer)

    go func() {
        log.Printf("Link Service gRPC server running on port %s...", cfg.GRPCPort)
        if err := grpcServer.Serve(lis); err != nil {
            logger.Fatalf("Failed to serve gRPC: %v", err)
        }
    }()

    // Graceful shutdown
    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
    <-stop

    log.Println("Shutting down Link Service gracefully...")
    healthServer.Shutdown()
    // GracefulStop dulu supaya tidak ada request baru yang publish event setelah publisher ditutup
    grpcServer.GracefulStop()
    opsCtx, cancelOps := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancelOps()
    _ = stopOps(opsCtx)

    flushCtx, cancelFlush := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancelFlush()
    if err := publisher.Close(flushCtx); err != nil {
        log.Printf("Failed to close kafka publisher: %v", err)
    }
    log.Println("Link Service stopped.")
}
