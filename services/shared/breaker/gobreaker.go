package breaker

import (
    "errors"
    "time"

    "github.com/sony/gobreaker"
)

type goBreakerWrapper struct {
    cb *gobreaker.CircuitBreaker
}

// NewGoBreaker membuat instance circuit breaker baru per dependency
func NewGoBreaker(name string) Breaker {
    settings := gobreaker.Settings{
        Name:        name,
        MaxRequests: 3,                 // Jumlah request sukses sblm state dari half-open ke closed
        Interval:    30 * time.Second,  // Window reset counter saat closed (0 = tidak pernah reset → rasio dihitung sejak start)
        Timeout:     5 * time.Second,   // Waktu state open sblm menjadi half-open
        ReadyToTrip: func(counts gobreaker.Counts) bool {
            // Breaker terbuka (Trip) jika tingkat kegagalan > 60% dengan minimal 5 request
            failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
            return counts.Requests >= 5 && failureRatio >= 0.6
        },
    }

    return &goBreakerWrapper{
        cb: gobreaker.NewCircuitBreaker(settings),
    }
}

func (g *goBreakerWrapper) Execute(req func() (interface{}, error)) (interface{}, error) {
    res, err := g.cb.Execute(req)
    // Samakan error "breaker menolak request" ke ErrOpen supaya pemanggil tidak bergantung ke gobreaker
    if errors.Is(err, gobreaker.ErrOpenState) || errors.Is(err, gobreaker.ErrTooManyRequests) {
        return nil, ErrOpen
    }
    return res, err
}
