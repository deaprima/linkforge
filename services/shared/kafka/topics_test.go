package kafka

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

func TestEnsureTopic_UnreachableBroker(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := EnsureTopic(ctx, "127.0.0.1:1", "t", 1)
	if err == nil || !strings.Contains(err.Error(), "create topic") {
		t.Errorf("want create topic error, got %v", err)
	}
}

// Integration: butuh broker sungguhan, mis. KAFKA_TEST_BROKERS=localhost:9092.
func TestEnsureTopic_IsIdempotent(t *testing.T) {
	broker := os.Getenv("KAFKA_TEST_BROKERS")
	if broker == "" {
		t.Skip("KAFKA_TEST_BROKERS not set; skipping Kafka integration test")
	}
	broker = strings.Split(broker, ",")[0]
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topic := "test.ensure-topic." + time.Now().Format("150405.000")
	for i := 0; i < 2; i++ { // panggilan kedua: topic sudah ada, tetap sukses
		if err := EnsureTopic(ctx, broker, topic, 3); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
}
