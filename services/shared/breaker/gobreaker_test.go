package breaker

import (
	"errors"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

var errFail = errors.New("dependency failed")

func run(b Breaker, fail bool) error {
	_, err := b.Execute(func() (interface{}, error) {
		if fail {
			return nil, errFail
		}
		return "ok", nil
	})
	return err
}

func TestGoBreaker_PassesThroughResults(t *testing.T) {
	b := NewGoBreaker(t.Name())
	res, err := b.Execute(func() (interface{}, error) { return 42, nil })
	if err != nil || res != 42 {
		t.Fatalf("got %v, %v", res, err)
	}
	if err := run(b, true); !errors.Is(err, errFail) {
		t.Fatalf("want original error, got %v", err)
	}
}

func TestGoBreaker_TripsAtFailureRatio(t *testing.T) {
	b := NewGoBreaker(t.Name())
	for i := 0; i < 5; i++ {
		_ = run(b, true)
	}
	called := false
	_, err := b.Execute(func() (interface{}, error) { called = true; return nil, nil })
	if !errors.Is(err, ErrOpen) {
		t.Fatalf("want ErrOpen after 5/5 failures, got %v", err)
	}
	if called {
		t.Error("open breaker must not call the dependency")
	}
}

func TestGoBreaker_StaysClosed(t *testing.T) {
	t.Run("below minimum request count", func(t *testing.T) {
		b := NewGoBreaker(t.Name())
		for i := 0; i < 4; i++ {
			_ = run(b, true)
		}
		if err := run(b, false); err != nil {
			t.Fatalf("4 failures (< 5 requests) must not trip: %v", err)
		}
	})
	t.Run("below failure ratio", func(t *testing.T) {
		b := NewGoBreaker(t.Name())
		// 7 sukses + 3 gagal = 30% < 60%
		for i := 0; i < 7; i++ {
			_ = run(b, false)
		}
		for i := 0; i < 3; i++ {
			_ = run(b, true)
		}
		if err := run(b, false); err != nil {
			t.Fatalf("30%% failure ratio must not trip: %v", err)
		}
	})
}

func TestGoBreaker_ExportsStateMetrics(t *testing.T) {
	name := t.Name()
	b := NewGoBreaker(name)
	if got := testutil.ToFloat64(stateGauge.WithLabelValues(name)); got != 0 {
		t.Fatalf("initial state = %v, want 0 (closed)", got)
	}

	for i := 0; i < 5; i++ {
		_ = run(b, true)
	}
	if got := testutil.ToFloat64(stateGauge.WithLabelValues(name)); got != 2 {
		t.Errorf("state after trip = %v, want 2 (open)", got)
	}
	if got := testutil.ToFloat64(transitions.WithLabelValues(name, "open")); got != 1 {
		t.Errorf("open transitions = %v, want 1", got)
	}
}
