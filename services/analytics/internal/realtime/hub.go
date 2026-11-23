// Package realtime meneruskan klik baru ke subscriber SubscribeRealtime (dashboard WebSocket).
package realtime

import (
	"sync"
	"sync/atomic"
	"time"
)

const subscriberBuffer = 64

// Click adalah event realtime internal; UserID dipakai untuk filter, tidak dikirim ke client.
type Click struct {
	UserID    string
	LinkID    string
	Alias     string
	Country   string
	Browser   string
	Device    string
	ClickedAt time.Time
}

type Subscription struct {
	C       <-chan Click
	c       chan Click
	userID  string
	linkIDs map[string]bool // kosong = semua link milik user
}

// Hub adalah fan-out in-memory. Catatan: setiap instance Analytics hanya menerima event dari
// partisi Kafka miliknya, jadi kalau di-scale > 1 instance, realtime perlu broadcast lintas
// instance (mis. Redis pub/sub) — belum dibutuhkan selama Analytics single instance.
type Hub struct {
	mu      sync.RWMutex
	subs    map[*Subscription]struct{}
	dropped atomic.Int64
}

func NewHub() *Hub {
	return &Hub{subs: make(map[*Subscription]struct{})}
}

func (h *Hub) Subscribe(userID string, linkIDs []string) *Subscription {
	c := make(chan Click, subscriberBuffer)
	s := &Subscription{C: c, c: c, userID: userID, linkIDs: make(map[string]bool, len(linkIDs))}
	for _, id := range linkIDs {
		s.linkIDs[id] = true
	}
	h.mu.Lock()
	h.subs[s] = struct{}{}
	h.mu.Unlock()
	return s
}

func (h *Hub) Unsubscribe(s *Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.subs[s]; ok {
		delete(h.subs, s)
		close(s.c)
	}
}

// Publish tidak pernah memblokir consumer Kafka: subscriber yang lambat kehilangan event
// (dashboard realtime boleh lossy; data historis tetap lengkap di database).
func (h *Hub) Publish(click Click) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for s := range h.subs {
		if s.userID != click.UserID || (len(s.linkIDs) > 0 && !s.linkIDs[click.LinkID]) {
			continue
		}
		select {
		case s.c <- click:
		default:
			h.dropped.Add(1)
		}
	}
}

func (h *Hub) Subscribers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.subs)
}

func (h *Hub) Dropped() int64 { return h.dropped.Load() }
