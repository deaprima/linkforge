// Package kafka berisi konstanta topic dan helper yang dipakai bersama oleh producer & consumer.
package kafka

import (
	"context"
	"errors"
	"fmt"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

// TopicLinkClicked membawa events.LinkClicked (lihat shared/proto/events/link_events.proto).
const TopicLinkClicked = "link.clicked"

const readyPollInterval = 200 * time.Millisecond

// EnsureTopic membuat topic kalau belum ada, lalu MENUNGGU sampai setiap partisi punya leader.
// Aman dipanggil berulang (idempotent). Partisi > 1 supaya consumer bisa di-scale horizontal;
// replication 1 karena single broker.
//
// Menunggu leader itu penting: tepat setelah CreateTopics, metadata topic belum tersebar.
// Producer yang langsung menulis akan gagal di sebagian partisi, dan consumer group yang
// langsung join bisa mendapat assignment sebagian/kosong.
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
	return waitForLeaders(ctx, client, topic)
}

func waitForLeaders(ctx context.Context, client *kafkago.Client, topic string) error {
	for {
		ready, err := topicReady(ctx, client, topic)
		if ready {
			return nil
		}
		select {
		case <-ctx.Done():
			if err == nil {
				err = errors.New("partitions have no leader yet")
			}
			return fmt.Errorf("topic %s not ready: %w", topic, err)
		case <-time.After(readyPollInterval):
		}
	}
}

func topicReady(ctx context.Context, client *kafkago.Client, topic string) (bool, error) {
	meta, err := client.Metadata(ctx, &kafkago.MetadataRequest{Topics: []string{topic}})
	if err != nil {
		return false, err
	}
	for _, t := range meta.Topics {
		if t.Name != topic {
			continue
		}
		if t.Error != nil {
			return false, t.Error
		}
		if len(t.Partitions) == 0 {
			return false, nil
		}
		for _, p := range t.Partitions {
			if p.Error != nil || p.Leader.Host == "" {
				return false, p.Error
			}
		}
		return true, nil
	}
	return false, nil
}
