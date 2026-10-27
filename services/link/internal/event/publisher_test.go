package event

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	eventspb "github.com/deaprima/linkforge/services/shared/proto/events"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
)

// fakeWriter mencatat pesan yang ditulis; block bisa dipakai untuk menahan worker.
type fakeWriter struct {
	mu      sync.Mutex
	batches [][]kafkago.Message
	err     error
	block   chan struct{}
	closed  bool
}

func (w *fakeWriter) WriteMessages(_ context.Context, msgs ...kafkago.Message) error {
	if w.block != nil {
		<-w.block
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.batches = append(w.batches, append([]kafkago.Message(nil), msgs...))
	return w.err
}

func (w *fakeWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return nil
}

func (w *fakeWriter) messages() []kafkago.Message {
	w.mu.Lock()
	defer w.mu.Unlock()
	var all []kafkago.Message
	for _, b := range w.batches {
		all = append(all, b...)
	}
	return all
}

func click(id string) *eventspb.LinkClicked {
	return &eventspb.LinkClicked{EventId: id, LinkId: "link-" + id}
}

func closeWithin(t *testing.T, p *KafkaPublisher, d time.Duration) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	if err := p.Close(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestPublisher_EncodesEventWithLinkIDKey(t *testing.T) {
	w := &fakeWriter{}
	p := NewKafkaPublisher(w, 10)
	p.PublishClick(&eventspb.LinkClicked{EventId: "e1", LinkId: "link-42", Ip: "1.2.3.4"})
	closeWithin(t, p, time.Second)

	msgs := w.messages()
	if len(msgs) != 1 {
		t.Fatalf("wrote %d messages, want 1", len(msgs))
	}
	if string(msgs[0].Key) != "link-42" {
		t.Errorf("key = %q, want link id (keeps per-link ordering)", msgs[0].Key)
	}
	var decoded eventspb.LinkClicked
	if err := proto.Unmarshal(msgs[0].Value, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.EventId != "e1" || decoded.Ip != "1.2.3.4" {
		t.Errorf("decoded = %+v", &decoded)
	}
	if !w.closed {
		t.Error("writer must be closed")
	}
}

func TestPublisher_BatchesQueuedEvents(t *testing.T) {
	w := &fakeWriter{block: make(chan struct{})}
	p := NewKafkaPublisher(w, 1000)

	// Event pertama menahan worker di WriteMessages; sisanya menumpuk di antrian
	p.PublishClick(click("first"))
	time.Sleep(20 * time.Millisecond)
	for i := range 250 {
		p.PublishClick(click(string(rune('a' + i%26))))
	}
	close(w.block)
	closeWithin(t, p, 2*time.Second)

	if got := len(w.messages()); got != 251 {
		t.Fatalf("wrote %d messages, want 251", got)
	}
	for i, b := range w.batches {
		if len(b) > maxBatchSize {
			t.Errorf("batch %d has %d messages, max %d", i, len(b), maxBatchSize)
		}
	}
	if len(w.batches) > 5 {
		t.Errorf("%d batches for 251 queued events — events are not being batched", len(w.batches))
	}
}

func TestPublisher_NeverBlocksWhenBufferIsFull(t *testing.T) {
	w := &fakeWriter{block: make(chan struct{})} // Kafka "hang"
	p := NewKafkaPublisher(w, 5)

	start := time.Now()
	for i := range 100 {
		p.PublishClick(click(string(rune('a' + i%26))))
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("PublishClick blocked for %v while kafka was stuck", elapsed)
	}
	// 1 sedang ditulis worker + 5 di buffer, sisanya dibuang
	if dropped := p.Dropped(); dropped < 90 {
		t.Errorf("dropped = %d, want >= 90", dropped)
	}
	close(w.block)
	closeWithin(t, p, time.Second)
}

func TestPublisher_CountsFailedWrites(t *testing.T) {
	w := &fakeWriter{err: errors.New("kafka: broker not available")}
	p := NewKafkaPublisher(w, 10)
	p.PublishClick(click("1"))
	p.PublishClick(click("2"))
	closeWithin(t, p, time.Second)

	if p.Failed() != 2 {
		t.Errorf("failed = %d, want 2", p.Failed())
	}
}

func TestPublisher_CloseRespectsTimeout(t *testing.T) {
	w := &fakeWriter{block: make(chan struct{})}
	p := NewKafkaPublisher(w, 10)
	p.PublishClick(click("stuck"))

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_ = p.Close(ctx)
	if time.Since(start) > 500*time.Millisecond {
		t.Errorf("Close did not respect the shutdown timeout")
	}
	close(w.block)
}

func TestNewKafkaWriter(t *testing.T) {
	w := NewKafkaWriter([]string{"localhost:9092"}, "link.clicked")
	defer w.Close()
	if w.Topic != "link.clicked" {
		t.Errorf("topic = %q", w.Topic)
	}
	if _, ok := w.Balancer.(*kafkago.Hash); !ok {
		t.Errorf("balancer = %T, want *kafka.Hash (partition by link_id)", w.Balancer)
	}
	// Default kafka-go = RequireNone (acks=0): pesan bisa hilang tanpa error. Jangan sampai regress.
	if w.RequiredAcks != kafkago.RequireOne {
		t.Errorf("RequiredAcks = %v, want RequireOne — acks=0 silently drops events", w.RequiredAcks)
	}
}
