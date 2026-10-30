package main

import (
	"context"
	"log"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/deaprima/linkforge/services/analytics/internal/config"
	"github.com/deaprima/linkforge/services/analytics/internal/consumer"
	grpcDelivery "github.com/deaprima/linkforge/services/analytics/internal/delivery/grpc"
	"github.com/deaprima/linkforge/services/analytics/internal/enrich"
	"github.com/deaprima/linkforge/services/analytics/internal/entity"
	"github.com/deaprima/linkforge/services/analytics/internal/realtime"
	"github.com/deaprima/linkforge/services/analytics/internal/repository"
	"github.com/deaprima/linkforge/services/analytics/internal/service"
	"github.com/deaprima/linkforge/services/shared/database"
	"github.com/deaprima/linkforge/services/shared/kafka"
	"github.com/deaprima/linkforge/services/shared/logger"
	"github.com/deaprima/linkforge/services/shared/metrics"
	"github.com/deaprima/linkforge/services/shared/ops"
	pb"github.com/deaprima/linkforge/services/shared/proto/analytics"
	"github.com/joho/godotenv"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const clickTopicPartitions = 3

func main() {
	envErr := godotenv.Load("../.env")
	logger.Setup("analytics") // setelah .env dimuat supaya LOG_LEVEL / LOG_FORMAT terbaca
	if envErr != nil {
		slog.Warn(".env file not found, relying on system environment variables")
	}

	log.Println("Starting Analytics Service..")

	cfg := config.LoadConfig()

	db, err := database.ConnectPostgres(cfg.DatabaseDSN, database.DebugFromEnv())
	if err != nil {
		logger.Fatalf("Failed to connect to database: %v", err)
	}

	log.Println("Running AutoMigration for Analytics Service...")
	if err := db.AutoMigrate(&entity.LinkClick{}, &entity.LinkDailyStat{}); err != nil {
		logger.Fatalf("Database migration failed: %v", err)
	}
	log.Println("Database migration completed successfully!")

	// GeoIP opsional — tanpa file GeoLite2, country dicatat sebagai "ZZ" (unknown)
	var geo enrich.CountryLookup
	if cfg.GeoIPDBPath != "" {
		g, err := enrich.OpenGeoLite2(cfg.GeoIPDBPath)
		if err != nil {
			logger.Fatalf("Failed to open GeoIP database: %v", err)
		}
		defer g.Close()
		geo = g
		log.Printf("GeoIP enabled (%s)", cfg.GeoIPDBPath)
	} else {
		log.Println("Warning: GEOIP_DB_PATH not set, country will be recorded as ZZ (unknown)")
	}

	// Topic dibuat di sini juga, supaya Analytics bisa start sebelum Link Service
	topicCtx, cancelTopic := context.WithTimeout(context.Background(), 10*time.Second)
	if err := kafka.EnsureTopic(topicCtx, cfg.KafkaBrokers[0], kafka.TopicLinkClicked, clickTopicPartitions); err != nil {
		log.Printf("Warning: could not ensure kafka topic %q: %v", kafka.TopicLinkClicked, err)
	}
	cancelTopic()

	reader := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     cfg.KafkaBrokers,
		GroupID:     cfg.KafkaGroupID,
		Topic:       kafka.TopicLinkClicked,
		StartOffset: kafkago.FirstOffset, // group baru membaca dari awal, tidak ada klik yang terlewat
		MaxWait:     500 * time.Millisecond,
		// Tanpa ini, member yang join saat metadata topic belum siap bisa mendapat assignment kosong selamanya
		WatchPartitionChanges: true,
		// CommitInterval 0 = commit synchronous; offset hanya maju setelah klik tersimpan
	})

	// Dependency injection
	clickRepo := repository.NewClickRepository(db)
	hub := realtime.NewHub()
	analyticsService := service.NewAnalyticsService(clickRepo)
	analyticsHandler := grpcDelivery.NewAnalyticsHandler(analyticsService, hub)
	clickConsumer := consumer.New(reader, clickRepo, enrich.New(geo), hub)

	registerConsumerMetrics(reader, clickConsumer, kafka.TopicLinkClicked, cfg.KafkaGroupID)

	consumerCtx, stopConsumer := context.WithCancel(context.Background())
	var consumerDone sync.WaitGroup
	consumerDone.Add(1)
	go func() {
		defer consumerDone.Done()
		log.Printf("Consuming %q as group %q...", kafka.TopicLinkClicked, cfg.KafkaGroupID)
		clickConsumer.Run(consumerCtx)
	}()

	lis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		logger.Fatalf("Failed to listen on port %s: %v", cfg.GRPCPort, err)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(metrics.UnaryServerInterceptor()),
		grpc.ChainStreamInterceptor(metrics.StreamServerInterceptor()),
	)
	stopOps := ops.Serve(ops.AddrFromEnv("ANALYTICS_METRICS_ADDR", ":9103"))
	pb.RegisterAnalyticsServiceServer(grpcServer, analyticsHandler)

	// Standard gRPC health check, dipakai Gateway untuk /readyz
	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(grpcServer, healthServer)

	go func() {
		log.Printf("Analytics Service gRPC server running on port %s...", cfg.GRPCPort)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Fatalf("Failed to serve gRPC: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down Analytics Service gracefully...")
	healthServer.Shutdown()

	// Consumer dihentikan dulu: pesan yang sedang diproses tapi belum di-commit akan dikirim ulang
	// ke instance berikutnya dan di-skip sebagai duplikat — tidak ada klik yang hilang atau dobel.
	stopConsumer()
	consumerDone.Wait()
	if err := reader.Close(); err != nil {
		log.Printf("Failed to close kafka reader: %v", err)
	}
	stats := clickConsumer.Stats()
	log.Printf("Consumer stopped: processed=%d duplicates=%d skipped=%d", stats.Processed, stats.Duplicates, stats.Skipped)

	// Stream SubscribeRealtime tidak pernah selesai sendiri; beri batas waktu sebelum dipaksa berhenti
	stopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		grpcServer.Stop()
	}
	opsCtx, cancelOps := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelOps()
	_ = stopOps(opsCtx)
	log.Println("Analytics Service stopped.")
}
