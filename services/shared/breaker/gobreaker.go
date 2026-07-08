package breaker

import (
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
        Interval:    0,                 // Interval reset cyclic
        Timeout:     5,                 // Waktu (detik) state open sblm menjadi half-open
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
    return g.cb.Execute(req)
}