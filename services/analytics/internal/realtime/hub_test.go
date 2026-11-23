package realtime

import (
	"testing"
	"time"
)

func receive(t *testing.T, s *Subscription) (Click, bool) {
	t.Helper()
	select {
	case c, ok := <-s.C:
		return c, ok
	case <-time.After(50 * time.Millisecond):
		return Click{}, false
	}
}

func TestHub_FiltersByUserAndLinks(t *testing.T) {
	h := NewHub()
	all := h.Subscribe("user-1", nil)
	onlyA := h.Subscribe("user-1", []string{"link-a"})
	other := h.Subscribe("user-2", nil)

	h.Publish(Click{UserID: "user-1", LinkID: "link-b"})

	if c, ok := receive(t, all); !ok || c.LinkID != "link-b" {
		t.Error("subscriber for all links must receive the click")
	}
	if _, ok := receive(t, onlyA); ok {
		t.Error("link filter must exclude link-b")
	}
	if _, ok := receive(t, other); ok {
		t.Error("other users must never receive the click")
	}

	h.Publish(Click{UserID: "user-1", LinkID: "link-a"})
	if c, ok := receive(t, onlyA); !ok || c.LinkID != "link-a" {
		t.Error("link filter must include link-a")
	}
}

func TestHub_SlowSubscriberDoesNotBlock(t *testing.T) {
	h := NewHub()
	h.Subscribe("user-1", nil) // tidak pernah dibaca

	start := time.Now()
	for range subscriberBuffer + 50 {
		h.Publish(Click{UserID: "user-1"})
	}
	if time.Since(start) > 100*time.Millisecond {
		t.Fatal("Publish blocked on a slow subscriber")
	}
	if h.Dropped() != 50 {
		t.Errorf("dropped = %d, want 50", h.Dropped())
	}
}

func TestHub_Unsubscribe(t *testing.T) {
	h := NewHub()
	s := h.Subscribe("user-1", nil)
	if h.Subscribers() != 1 {
		t.Fatal("want 1 subscriber")
	}
	h.Unsubscribe(s)
	h.Unsubscribe(s) // idempotent, tidak panic karena close ganda

	if h.Subscribers() != 0 {
		t.Error("subscriber not removed")
	}
	if _, ok := <-s.C; ok {
		t.Error("channel must be closed after unsubscribe")
	}
	h.Publish(Click{UserID: "user-1"}) // tidak panic setelah unsubscribe
}
