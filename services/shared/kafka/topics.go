// Package kafka berisi konstanta topic dan helper yang dipakai bersama oleh producer & consumer.
package kafka

import (
	"context"
	"errors"
	"fmt"

	kafkago "github.com/segmentio/kafka-go"
)

// TopicLinkClicked membawa events.LinkClicked (lihat shared/proto/events/link_events.proto).
const TopicLinkClicked = "link.clicked"

// EnsureTopic membuat topic kalau belum ada. Aman dipanggil berulang (idempotent).
// Partisi > 1 supaya consumer bisa di-scale horizontal; replication 1 karena single broker.
//
// Memakai kafka.Client (negosiasi versi protokol), bukan kafka.Conn.CreateTopics yang
// masih memakai CreateTopics v0 — versi itu sudah dihapus di Kafka 4.x (KIP-896).
func EnsureTopic(ctx context.Context, broker, topic string, partitions int) error {
	client := &kafkago.Client{Addr: kafkago.TCP(broker)}
	res, err := client.CreateTopics(ctx, &kafkago.CreateTopicsRequest{
		Topics: []kafkago.TopicConfig{{
			Topic:             topic,
			NumPartitions:     partitions,
			ReplicationFactor: 1,
		}},
	})
	if err != nil {
		return fmt.Errorf("create topic %s: %w", topic, err)
	}
	if err := res.Errors[topic]; err != nil && !errors.Is(err, kafkago.TopicAlreadyExists) {
		return fmt.Errorf("create topic %s: %w", topic, err)
	}
	return nil
}
