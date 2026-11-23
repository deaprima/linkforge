package breaker

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/sony/gobreaker"
)

// circuit_breaker_state: 0 = closed, 1 = half-open, 2 = open.
var (
	stateGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "circuit_breaker_state",
		Help: "State circuit breaker per dependency: 0 closed, 1 half-open, 2 open.",
	}, []string{"name"})

	transitions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "circuit_breaker_transitions_total",
		Help: "Perpindahan state circuit breaker.",
	}, []string{"name", "to"})
)

func stateValue(s gobreaker.State) float64 {
	switch s {
	case gobreaker.StateHalfOpen:
		return 1
	case gobreaker.StateOpen:
		return 2
	default:
		return 0
	}
}
