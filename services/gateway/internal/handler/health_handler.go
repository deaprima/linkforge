package handler

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const readinessTimeout = 2 * time.Second

// CheckFunc mengembalikan nil kalau dependency sehat.
type CheckFunc func(ctx context.Context) error

// HealthHandler melayani /healthz & /readyz. Tidak memakai response envelope
// (lihat API-CONTRACT.md bagian Ops & Observability).
type HealthHandler struct {
	checks map[string]CheckFunc
}

func NewHealthHandler(checks map[string]CheckFunc) *HealthHandler {
	return &HealthHandler{checks: checks}
}

// GET /healthz — liveness: proses Gateway hidup
func (h *HealthHandler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GET /readyz — readiness: semua dependency (Auth, Link, Redis) bisa dijangkau
func (h *HealthHandler) Readyz(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), readinessTimeout)
	defer cancel()

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		results = make(map[string]string, len(h.checks))
		healthy = true
	)
	for name, check := range h.checks {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := "ok"
			if err := check(ctx); err != nil {
				result = err.Error()
			}
			mu.Lock()
			defer mu.Unlock()
			results[name] = result
			if result != "ok" {
				healthy = false
			}
		}()
	}
	wg.Wait()

	if !healthy {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable", "checks": results})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "checks": results})
}
