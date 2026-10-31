package client

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deaprima/linkforge/services/shared/breaker"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newTestBase(t *testing.T, timeout time.Duration) *baseClient {
	return &baseClient{breaker: breaker.NewGoBreaker(t.Name()), timeout: timeout}
}

func TestCall_ReturnsResult(t *testing.T) {
	b := newTestBase(t, time.Second)
	got, err := call(context.Background(), b, func(context.Context) (string, error) { return "ok", nil })
	if err != nil || got != "ok" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestCall_BusinessErrorsDoNotTripBreaker(t *testing.T) {
	b := newTestBase(t, time.Second)
	for i := 0; i < 20; i++ {
		_, err := call(context.Background(), b, func(context.Context) (string, error) {
			return "", status.Error(codes.NotFound, "link not found")
		})
		if status.Code(err) != codes.NotFound {
			t.Fatalf("call %d: want NotFound passed through, got %v", i, err)
		}
	}
}

func TestCall_InfraErrorsTripBreaker(t *testing.T) {
	b := newTestBase(t, time.Second)
	calls := 0
	failing := func(context.Context) (string, error) {
		calls++
		return "", status.Error(codes.Unavailable, "connection refused")
	}

	// ReadyToTrip: minimal 5 request dengan failure ratio >= 60%
	for i := 0; i < 5; i++ {
		if _, err := call(context.Background(), b, failing); status.Code(err) != codes.Unavailable {
			t.Fatalf("call %d: want Unavailable, got %v", i, err)
		}
	}

	_, err := call(context.Background(), b, failing)
	if !errors.Is(err, breaker.ErrOpen) {
		t.Fatalf("want breaker.ErrOpen after trip, got %v", err)
	}
	if calls != 5 {
		t.Errorf("dependency called %d times, want 5 (open breaker must short-circuit)", calls)
	}
}

func TestCall_AppliesTimeout(t *testing.T) {
	b := newTestBase(t, 50*time.Millisecond)
	_, err := call(context.Background(), b, func(ctx context.Context) (string, error) {
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 50*time.Millisecond {
			t.Errorf("want deadline within 50ms, got %v (set=%v)", time.Until(deadline), ok)
		}
		<-ctx.Done()
		return "", status.FromContextError(ctx.Err()).Err()
	})
	if status.Code(err) != codes.DeadlineExceeded {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}
}

func TestIsInfraError(t *testing.T) {
	tests := map[codes.Code]bool{
		codes.Unavailable:        true,
		codes.DeadlineExceeded:   true,
		codes.Internal:           true,
		codes.Unknown:            true,
		codes.ResourceExhausted:  true,
		codes.Aborted:            true,
		codes.NotFound:           false,
		codes.AlreadyExists:      false,
		codes.InvalidArgument:    false,
		codes.Unauthenticated:    false,
		codes.PermissionDenied:   false,
		codes.FailedPrecondition: false,
		codes.Canceled:           false,
	}
	for code, want := range tests {
		if got := isInfraError(status.Error(code, "x")); got != want {
			t.Errorf("isInfraError(%s) = %v, want %v", code, got, want)
		}
	}
}

func TestClients_CheckFailsWhenServiceIsDown(t *testing.T) {
	// Port 1 tidak pernah listen — koneksi lazy, jadi constructor tetap sukses
	auth, err := NewAuthClient("127.0.0.1:1", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer auth.Close()
	link, err := NewLinkClient("127.0.0.1:1", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer link.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	if auth.Check(ctx) == nil || link.Check(ctx) == nil {
		t.Error("Check must fail when the service is unreachable")
	}
}

func TestDependencyName(t *testing.T) {
	cases := map[string]string{"gateway->auth": "auth", "gateway->link": "link", "standalone": "standalone"}
	for in, want := range cases {
		if got := dependencyName(in); got != want {
			t.Errorf("dependencyName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGRPCTarget(t *testing.T) {
	cases := map[string]string{
		"auth:50051":             "passthrough:///auth:50051",
		"localhost:50051":        "passthrough:///localhost:50051",
		"dns:///auth:50051":      "dns:///auth:50051",
		"unix:///tmp/auth.sock":  "unix:///tmp/auth.sock",
	}
	for in, want := range cases {
		if got := grpcTarget(in); got != want {
			t.Errorf("grpcTarget(%q) = %q, want %q", in, got, want)
		}
	}
}
