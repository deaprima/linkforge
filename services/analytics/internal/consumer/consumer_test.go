package consumer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/deaprima/linkforge/services/analytics/internal/enrich"
	"github.com/deaprima/linkforge/services/analytics/internal/entity"
	"github.com/deaprima/linkforge/services/analytics/internal/realtime"
	eventspb "github.com/deaprima/linkforge/services/shared/proto/events"
	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// fakeReader menyajikan pesan berurutan lalu memblokir sampai ctx dibatalkan.
type fakeReader struct {
	mu        sync.Mutex
	msgs      []kafkago.Message
	pos       int
	committed []int64
	fetchErrs int // jumlah FetchMessage pertama yang gagal
	commitErr error
}

func (r *fakeReader) FetchMessage(ctx context.Context) (kafkago.Message, error) {
	r.mu.Lock()
	if r.fetchErrs > 0 {
		r.fetchErrs--
		r.mu.Unlock()
		return kafkago.Message{}, errors.New("broker not available")
	}
	if r.pos < len(r.msgs) {
		m := r.msgs[r.pos]
		r.pos++
		r.mu.Unlock()
		return m, nil
	}
	r.mu.Unlock()
	<-ctx.Done()
	return kafkago.Message{}, ctx.Err()
}

func (r *fakeReader) CommitMessages(_ context.Context, msgs ...kafkago.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range msgs {
		r.committed = append(r.committed, m.Offset)
	}
	return r.commitErr
}

func (r *fakeReader) commits() []int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]int64(nil), r.committed...)
}

// fakeStore idempotent berdasarkan event_id, dengan opsi gagal N kali pertama.
type fakeStore struct {
	mu       sync.Mutex
	clicks   map[uuid.UUID]*entity.LinkClick
	failures int
	calls    int
}

func newFakeStore() *fakeStore { return &fakeStore{clicks: map[uuid.UUID]*entity.LinkClick{}} }

func (s *fakeStore) SaveClick(_ context.Context, c *entity.LinkClick) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.failures > 0 {
		s.failures--
		return false, errors.New("db down")
	}
	if _, dup := s.clicks[c.EventID]; dup {
		return false, nil
	}
	s.clicks[c.EventID] = c
	return true, nil
}

func (s *fakeStore) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.clicks)
}

func eventMsg(t *testing.T, offset int64, ev *eventspb.LinkClicked) kafkago.Message {
	t.Helper()
	b, err := proto.Marshal(ev)
	if err != nil {
		t.Fatal(err)
	}
	return kafkago.Message{Offset: offset, Key: []byte(ev.LinkId), Value: b}
}

func validEvent(eventID string) *eventspb.LinkClicked {
	return &eventspb.LinkClicked{
		EventId:   eventID,
		LinkId:    "7c9e6679-7425-40de-944b-e07fc1f90ae7",
		UserId:    "user-1",
		Alias:     "promo",
		Ip:        "127.0.0.1",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) Mobile/15E148",
		Referrer:  "https://twitter.com/",
		ClickedAt: timestamppb.New(time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)),
	}
}

// runUntil menjalankan consumer sampai cond terpenuhi (atau timeout), lalu menghentikannya.
func runUntil(t *testing.T, c *Consumer, cond func() bool) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx); close(done) }()

	deadline := time.After(2 * time.Second)
	for !cond() {
		select {
		case <-deadline:
			cancel()
			<-done
			t.Fatal("condition not reached before timeout")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	<-done
}

func newTestConsumer(r *fakeReader, s *fakeStore, hub *realtime.Hub) *Consumer {
	c := New(r, s, enrich.New(nil), hub)
	c.retryBackoff = time.Millisecond
	return c
}

func TestConsumer_StoresEnrichedClickAndCommits(t *testing.T) {
	id := uuid.NewString()
	r := &fakeReader{msgs: []kafkago.Message{eventMsg(t, 7, validEvent(id))}}
	s := newFakeStore()
	hub := realtime.NewHub()
	sub := hub.Subscribe("user-1", nil)
	c := newTestConsumer(r, s, hub)

	runUntil(t, c, func() bool { return len(r.commits()) == 1 })

	click := s.clicks[uuid.MustParse(id)]
	if click == nil {
		t.Fatal("click not stored")
	}
	if click.Alias != "promo" || click.Device != enrich.DeviceMobile || click.Country != enrich.UnknownCountry ||
		click.Referrer != "https://twitter.com/" || !click.ClickedAt.Equal(time.Date(2026, 10, 27, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("click = %+v", click)
	}
	if got := r.commits(); got[0] != 7 {
		t.Errorf("committed offsets = %v, want [7]", got)
	}

	select {
	case rt := <-sub.C:
		if rt.LinkID != click.LinkID.String() || rt.Device != enrich.DeviceMobile {
			t.Errorf("realtime click = %+v", rt)
		}
	default:
		t.Error("new click must be published to realtime hub")
	}
	if st := c.Stats(); st.Processed != 1 || st.Duplicates != 0 {
		t.Errorf("stats = %+v", st)
	}
}

func TestConsumer_DuplicateIsCommittedButNotRepublished(t *testing.T) {
	id := uuid.NewString()
	// Event yang sama dikirim dua kali (redelivery setelah restart)
	r := &fakeReader{msgs: []kafkago.Message{eventMsg(t, 1, validEvent(id)), eventMsg(t, 2, validEvent(id))}}
	s := newFakeStore()
	hub := realtime.NewHub()
	sub := hub.Subscribe("user-1", nil)
	c := newTestConsumer(r, s, hub)

	runUntil(t, c, func() bool { return len(r.commits()) == 2 })

	if s.count() != 1 {
		t.Errorf("stored %d clicks, want 1", s.count())
	}
	if st := c.Stats(); st.Processed != 1 || st.Duplicates != 1 {
		t.Errorf("stats = %+v", st)
	}
	<-sub.C
	select {
	case <-sub.C:
		t.Error("duplicate must not be published to realtime again")
	default:
	}
}

func TestConsumer_RetriesDatabaseErrorWithoutSkippingAhead(t *testing.T) {
	first, second := uuid.NewString(), uuid.NewString()
	r := &fakeReader{msgs: []kafkago.Message{eventMsg(t, 1, validEvent(first)), eventMsg(t, 2, validEvent(second))}}
	s := newFakeStore()
	s.failures = 3
	c := newTestConsumer(r, s, realtime.NewHub())

	runUntil(t, c, func() bool { return len(r.commits()) == 2 })

	if s.count() != 2 {
		t.Fatalf("stored %d clicks, want 2", s.count())
	}
	if got := r.commits(); got[0] != 1 || got[1] != 2 {
		t.Errorf("commits = %v, want [1 2] (in order, offset 1 only after it was saved)", got)
	}
	if s.calls != 5 {
		t.Errorf("SaveClick calls = %d, want 5 (3 failures + 2 successes)", s.calls)
	}
}

func TestConsumer_ShutdownDuringRetryDoesNotCommit(t *testing.T) {
	r := &fakeReader{msgs: []kafkago.Message{eventMsg(t, 1, validEvent(uuid.NewString()))}}
	s := newFakeStore()
	s.failures = 1 << 30 // DB mati terus
	c := newTestConsumer(r, s, realtime.NewHub())

	runUntil(t, c, func() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.calls >= 3 })

	if len(r.commits()) != 0 {
		t.Error("offset must not be committed for an unsaved click (it must be redelivered)")
	}
}

func TestConsumer_SkipsAndCommitsPoisonMessages(t *testing.T) {
	noClickedAt := validEvent(uuid.NewString())
	noClickedAt.ClickedAt = nil
	badLink := validEvent(uuid.NewString())
	badLink.LinkId = "not-a-uuid"
	badEvent := validEvent("not-a-uuid")
	good := uuid.NewString()

	r := &fakeReader{msgs: []kafkago.Message{
		{Offset: 1, Value: []byte{0xff, 0xff, 0xff}}, // bukan protobuf valid
		eventMsg(t, 2, badEvent),
		eventMsg(t, 3, badLink),
		eventMsg(t, 4, noClickedAt),
		eventMsg(t, 5, validEvent(good)),
	}}
	s := newFakeStore()
	c := newTestConsumer(r, s, realtime.NewHub())

	runUntil(t, c, func() bool { return len(r.commits()) == 5 })

	if s.count() != 1 || s.clicks[uuid.MustParse(good)] == nil {
		t.Errorf("only the valid event must be stored, got %d", s.count())
	}
	if st := c.Stats(); st.Skipped != 4 || st.Processed != 1 {
		t.Errorf("stats = %+v", st)
	}
}

func TestConsumer_RecoversFromFetchAndCommitErrors(t *testing.T) {
	r := &fakeReader{
		msgs:      []kafkago.Message{eventMsg(t, 1, validEvent(uuid.NewString()))},
		fetchErrs: 2,
		commitErr: errors.New("coordinator not available"),
	}
	s := newFakeStore()
	c := newTestConsumer(r, s, realtime.NewHub())

	runUntil(t, c, func() bool { return len(r.commits()) == 1 })

	if s.count() != 1 {
		t.Errorf("stored %d clicks, want 1", s.count())
	}
}
