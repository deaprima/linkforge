package consumer

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deaprima/linkforge/services/analytics/internal/enrich"
	"github.com/deaprima/linkforge/services/analytics/internal/entity"
	"github.com/deaprima/linkforge/services/analytics/internal/realtime"
	"github.com/deaprima/linkforge/services/analytics/internal/repository"
	"github.com/deaprima/linkforge/services/shared/kafka"
	eventspb "github.com/deaprima/linkforge/services/shared/proto/events"
	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Integration test alur event streaming end-to-end: Kafka sungguhan → consumer → Postgres sungguhan.
// Di-skip kecuali KAFKA_TEST_BROKERS dan ANALYTICS_TEST_DSN di-set, mis.:
//
//	KAFKA_TEST_BROKERS=localhost:9092 \
//	ANALYTICS_TEST_DSN="postgres://user:pass@localhost:5432/linkforge?sslmode=disable" \
//	go test ./analytics/internal/consumer/ -run Integration -v
//
// Setiap test memakai topic, consumer group, dan schema Postgres sendiri yang dibersihkan setelahnya.

type integrationEnv struct {
	broker string
	topic  string
	group  string
	db     *gorm.DB
	repo   repository.ClickRepository
}

func newIntegrationEnv(t *testing.T) *integrationEnv {
	t.Helper()
	broker, dsn := os.Getenv("KAFKA_TEST_BROKERS"), os.Getenv("ANALYTICS_TEST_DSN")
	if broker == "" || dsn == "" {
		t.Skip("KAFKA_TEST_BROKERS / ANALYTICS_TEST_DSN not set; skipping Kafka integration test")
	}
	suffix := strings.ReplaceAll(uuid.NewString()[:8], "-", "")
	env := &integrationEnv{
		broker: strings.Split(broker, ",")[0],
		topic:  "it-link-clicked-" + suffix,
		group:  "it-analytics-" + suffix,
	}

	// Schema Postgres sementara
	schema := "analytics_it_" + suffix
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatal(err)
	}
	sep := "?"
	if strings.Contains(dsn, "?") {
		sep = "&"
	}
	env.db, err = gorm.Open(postgres.Open(dsn+sep+"search_path="+schema), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.db.AutoMigrate(&entity.LinkClick{}, &entity.LinkDailyStat{}); err != nil {
		t.Fatal(err)
	}
	env.repo = repository.NewClickRepository(env.db)

	// Topic sementara, 3 partisi seperti produksi
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := kafka.EnsureTopic(ctx, env.broker, env.topic, 3); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		client := &kafkago.Client{Addr: kafkago.TCP(env.broker)}
		_, _ = client.DeleteTopics(context.Background(), &kafkago.DeleteTopicsRequest{Topics: []string{env.topic}})
		_, _ = client.DeleteGroups(context.Background(), &kafkago.DeleteGroupsRequest{GroupIDs: []string{env.group}})
		if sqlDB, err := env.db.DB(); err == nil {
			sqlDB.Close()
		}
		admin.Exec("DROP SCHEMA " + schema + " CASCADE")
		if sqlDB, err := admin.DB(); err == nil {
			sqlDB.Close()
		}
	})
	return env
}

// produce mengirim event seperti Link Service: protobuf, key = link_id, acks=1.
// RequiredAcks WAJIB di-set: default kafka-go adalah RequireNone (acks=0), yang membuang
// pesan tanpa error kalau partisi belum siap — test jadi gagal secara acak.
func (e *integrationEnv) produce(t *testing.T, events []*eventspb.LinkClicked) {
	t.Helper()
	w := &kafkago.Writer{Addr: kafkago.TCP(e.broker), Topic: e.topic, Balancer: &kafkago.Hash{},
		RequiredAcks: kafkago.RequireOne, BatchTimeout: 10 * time.Millisecond}
	defer w.Close()
	msgs := make([]kafkago.Message, len(events))
	for i, ev := range events {
		b, err := proto.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		msgs[i] = kafkago.Message{Key: []byte(ev.LinkId), Value: b}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := w.WriteMessages(ctx, msgs...); err != nil {
		t.Fatal(err)
	}
}

func (e *integrationEnv) newReader() *kafkago.Reader {
	// Konfigurasi sama dengan analytics/cmd/server/main.go
	return kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:               []string{e.broker},
		GroupID:               e.group,
		Topic:                 e.topic,
		StartOffset:           kafkago.FirstOffset,
		MaxWait:               200 * time.Millisecond,
		WatchPartitionChanges: true,
	})
}

func (e *integrationEnv) counts(t *testing.T) (clicks, distinctEvents, rollup int64) {
	t.Helper()
	e.db.Raw("SELECT COUNT(*) FROM link_clicks").Scan(&clicks)
	e.db.Raw("SELECT COUNT(DISTINCT event_id) FROM link_clicks").Scan(&distinctEvents)
	e.db.Raw("SELECT COALESCE(SUM(click_count), 0) FROM link_daily_stats").Scan(&rollup)
	return
}

func makeEvents(n int, links []string) []*eventspb.LinkClicked {
	events := make([]*eventspb.LinkClicked, n)
	base := time.Now().UTC().Add(-time.Hour)
	for i := range events {
		events[i] = &eventspb.LinkClicked{
			EventId:   uuid.NewString(),
			LinkId:    links[i%len(links)],
			UserId:    "user-1",
			Alias:     fmt.Sprintf("alias-%d", i%len(links)),
			Ip:        fmt.Sprintf("203.0.113.%d", i%250),
			UserAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/120.0",
			ClickedAt: timestamppb.New(base.Add(time.Duration(i) * time.Millisecond)),
		}
	}
	return events
}

// runConsumer menjalankan consumer sampai cond terpenuhi, lalu menghentikannya.
func runConsumer(t *testing.T, c *Consumer, reader *kafkago.Reader, cond func() bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); c.Run(ctx) }()

	deadline := time.Now().Add(60 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			cancel()
			wg.Wait()
			reader.Close()
			t.Fatalf("condition not reached within 60s (stats %+v)", c.Stats())
		}
		time.Sleep(50 * time.Millisecond)
	}
	cancel()
	wg.Wait()
	if err := reader.Close(); err != nil {
		t.Logf("reader close: %v", err)
	}
}

func TestIntegration_ProcessesEveryEventExactlyOnce(t *testing.T) {
	env := newIntegrationEnv(t)
	links := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	events := makeEvents(200, links)

	// Producer retry mengirim sebagian event dua kali — tetap harus tercatat sekali
	withDuplicates := append(append([]*eventspb.LinkClicked{}, events...), events[:25]...)
	env.produce(t, withDuplicates)

	reader := env.newReader()
	c := New(reader, env.repo, enrich.New(nil), realtime.NewHub())
	runConsumer(t, c, reader, func() bool { st := c.Stats(); return st.Processed+st.Duplicates >= int64(len(withDuplicates)) })

	clicks, distinct, rollup := env.counts(t)
	if clicks != 200 || distinct != 200 || rollup != 200 {
		t.Errorf("clicks=%d distinct=%d rollup=%d, want 200 each", clicks, distinct, rollup)
	}
	if st := c.Stats(); st.Processed != 200 || st.Duplicates != 25 {
		t.Errorf("stats = %+v, want processed=200 duplicates=25", st)
	}
}

// crashingReader meneruskan ke reader asli, tapi "mati" setelah N commit: commit berikutnya
// tidak pernah sampai ke Kafka — persis seperti proses yang crash setelah menulis ke DB
// tapi sebelum offset di-commit.
type crashingReader struct {
	*kafkago.Reader
	commitsBeforeCrash int64
	commits            atomic.Int64
}

func (r *crashingReader) CommitMessages(ctx context.Context, msgs ...kafkago.Message) error {
	if r.commits.Add(1) > r.commitsBeforeCrash {
		return nil // hilang: offset tidak maju
	}
	return r.Reader.CommitMessages(ctx, msgs...)
}

func TestIntegration_ConsumerRestartLosesAndDuplicatesNothing(t *testing.T) {
	env := newIntegrationEnv(t)
	events := makeEvents(300, []string{uuid.NewString(), uuid.NewString(), uuid.NewString()})
	env.produce(t, events)

	// Instance A: menyimpan 120 klik, tapi hanya 50 offset pertama yang sempat di-commit
	readerA := env.newReader()
	crashing := &crashingReader{Reader: readerA, commitsBeforeCrash: 50}
	a := New(crashing, env.repo, enrich.New(nil), realtime.NewHub())
	runConsumer(t, a, readerA, func() bool { return a.Stats().Processed >= 120 })

	// Hitung dari DB, bukan dari Stats(): A bisa dihentikan saat transaksi sedang commit —
	// baris sudah tersimpan di Postgres tapi SaveClick melihat context canceled. Itu benar
	// (at-least-once): klik tsb akan dikirim ulang ke B dan di-skip sebagai duplikat.
	rowsAfterA, _, _ := env.counts(t)
	const committedByA = 50
	t.Logf("instance A crashed: %d clicks stored, only %d offsets committed", rowsAfterA, committedByA)

	// Instance B (group yang sama) melanjutkan dari offset terakhir yang di-commit,
	// jadi menerima ulang tepat 300 - 50 = 250 pesan.
	redelivered := int64(len(events) - committedByA)
	readerB := env.newReader()
	b := New(readerB, env.repo, enrich.New(nil), realtime.NewHub())
	runConsumer(t, b, readerB, func() bool {
		st := b.Stats()
		return st.Processed+st.Duplicates >= redelivered
	})

	clicks, distinct, rollup := env.counts(t)
	if clicks != 300 || distinct != 300 || rollup != 300 {
		t.Fatalf("after restart: clicks=%d distinct=%d rollup=%d, want exactly 300 each (no loss, no duplicates)", clicks, distinct, rollup)
	}
	// Setiap klik yang disimpan A tapi offset-nya belum di-commit harus dikenali B sebagai duplikat
	st := b.Stats()
	if want := rowsAfterA - committedByA; st.Duplicates != want {
		t.Errorf("B duplicates = %d, want %d (stored by A but not committed)", st.Duplicates, want)
	}
	if st.Processed+st.Duplicates != redelivered {
		t.Errorf("B saw %d messages, want exactly %d redelivered", st.Processed+st.Duplicates, redelivered)
	}
	t.Logf("instance B: %+v", st)
}
