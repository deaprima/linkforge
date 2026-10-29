// Package event mengirim domain event Link Service ke Kafka.
package event

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	eventspb "github.com/deaprima/linkforge/services/shared/proto/events"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

const (
	defaultBufferSize = 10000
	maxBatchSize      = 100
	writeTimeout      = 5 * time.Second
)

// MessageWriter adalah subset *kafka.Writer yang dipakai publisher (memudahkan test).
type MessageWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafkago.Message) error
	Close() error
}

// KafkaPublisher mengirim event LinkClicked secara asynchronous.
//
// PublishClick tidak pernah memblokir redirect: event masuk ke buffer channel
// terbatas, lalu satu worker mengirimkannya ke Kafka per batch. Kalau buffer penuh
// (mis. Kafka mati cukup lama), event dibuang dan dihitung di Dropped() —
// availability redirect lebih penting daripada kelengkapan analytics.
type KafkaPublisher struct {
	writer  MessageWriter
	events  chan *eventspb.LinkClicked
	done    chan struct{}
	once    sync.Once
	dropped atomic.Int64
	failed  atomic.Int64
}

func NewKafkaWriter(brokers []string, topic string) *kafkago.Writer {
	return &kafkago.Writer{
		Addr:         kafkago.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kafkago.Hash{}, // key = link_id → partisi konsisten per link
		RequiredAcks: kafkago.RequireOne,
		BatchTimeout: 10 * time.Millisecond,
		MaxAttempts:  3,
	}
}

func NewKafkaPublisher(writer MessageWriter, bufferSize int) *KafkaPublisher {
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}
	p := &KafkaPublisher{
		writer: writer,
		events: make(chan *eventspb.LinkClicked, bufferSize),
		done:   make(chan struct{}),
	}
	go p.run()
	return p
}

// PublishClick memasukkan event ke antrian tanpa menunggu Kafka.
func (p *KafkaPublisher) PublishClick(ev *eventspb.LinkClicked) {
	select {
	case p.events <- ev:
	default:
		if n := p.dropped.Add(1); n == 1 || n%1000 == 0 {
			slog.Warn("click event buffer full, events dropped", "dropped_total", n)
		}
	}
}

// Dropped = event yang dibuang karena buffer penuh; Failed = event yang gagal dikirim ke Kafka.
func (p *KafkaPublisher) Dropped() int64 { return p.dropped.Load() }
func (p *KafkaPublisher) Failed() int64  { return p.failed.Load() }

func (p *KafkaPublisher) run() {
	defer close(p.done)
	batch := make([]kafkago.Message, 0, maxBatchSize)

	for ev := range p.events {
		batch = append(batch[:0], p.toMessage(ev))
		// Ambil event lain yang sudah antri tanpa menunggu, supaya dikirim sekaligus
	drain:
		for len(batch) < maxBatchSize {
			select {
			case next, ok := <-p.events:
				if !ok {
					break drain
				}
				batch = append(batch, p.toMessage(next))
			default:
				break drain
			}
		}
		p.write(batch)
	}
}

func (p *KafkaPublisher) toMessage(ev *eventspb.LinkClicked) kafkago.Message {
	value, err := proto.Marshal(ev)
	if err != nil {
		// Tidak mungkin terjadi untuk message proto yang valid; tetap jangan panic
		slog.Error("failed to marshal click event", "event_id", ev.GetEventId(), "error", err)
	}
	return kafkago.Message{Key: []byte(ev.GetLinkId()), Value: value}
}

func (p *KafkaPublisher) write(batch []kafkago.Message) {
	ctx, cancel := context.WithTimeout(context.Background(), writeTimeout)
	defer cancel()
	if err := p.writer.WriteMessages(ctx, batch...); err != nil {
		p.failed.Add(int64(len(batch)))
		slog.Error("failed to publish click events", "count", len(batch), "error", err)
	}
}

// Close berhenti menerima event, mengirim sisa antrian (dibatasi ctx), lalu menutup writer.
func (p *KafkaPublisher) Close(ctx context.Context) error {
	p.once.Do(func() { close(p.events) })
	select {
	case <-p.done:
	case <-ctx.Done():
		slog.Warn("shutdown timeout, click events not flushed", "pending", len(p.events))
	}
	return p.writer.Close()
}
