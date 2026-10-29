package metrics

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestShortMethod(t *testing.T) {
	cases := map[string]string{
		"/linkforge.auth.AuthService/Login": "AuthService/Login",
		"/grpc.health.v1.Health/Check":      "Health/Check",
		"/Plain/Method":                     "Plain/Method",
	}
	for in, want := range cases {
		if got := shortMethod(in); got != want {
			t.Errorf("shortMethod(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestUnaryServerInterceptor_CountsByCode(t *testing.T) {
	info := &grpc.UnaryServerInfo{FullMethod: "/linkforge.test.TestService/Do"}
	ic := UnaryServerInterceptor()

	ok := func(context.Context, any) (any, error) { return "x", nil }
	notFound := func(context.Context, any) (any, error) { return nil, status.Error(codes.NotFound, "nope") }

	before := testutil.ToFloat64(grpcServerHandled.WithLabelValues("TestService/Do", "OK"))
	beforeNF := testutil.ToFloat64(grpcServerHandled.WithLabelValues("TestService/Do", "NotFound"))

	if _, err := ic(context.Background(), nil, info, ok); err != nil {
		t.Fatal(err)
	}
	if _, err := ic(context.Background(), nil, info, notFound); status.Code(err) != codes.NotFound {
		t.Fatalf("error must pass through, got %v", err)
	}

	if got := testutil.ToFloat64(grpcServerHandled.WithLabelValues("TestService/Do", "OK")) - before; got != 1 {
		t.Errorf("OK delta = %v, want 1", got)
	}
	if got := testutil.ToFloat64(grpcServerHandled.WithLabelValues("TestService/Do", "NotFound")) - beforeNF; got != 1 {
		t.Errorf("NotFound delta = %v, want 1", got)
	}
	if n := testutil.CollectAndCount(grpcServerLatency); n == 0 {
		t.Error("latency histogram not recorded")
	}
}

func TestUnaryClientInterceptor_LabelsDependency(t *testing.T) {
	ic := UnaryClientInterceptor("auth-test")
	invoker := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		return status.Error(codes.Unavailable, "down")
	}
	before := testutil.ToFloat64(grpcClientHandled.WithLabelValues("auth-test", "AuthService/Login", "Unavailable"))

	err := ic(context.Background(), "/linkforge.auth.AuthService/Login", nil, nil, nil, invoker)
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("error must pass through, got %v", err)
	}
	if got := testutil.ToFloat64(grpcClientHandled.WithLabelValues("auth-test", "AuthService/Login", "Unavailable")) - before; got != 1 {
		t.Errorf("delta = %v, want 1", got)
	}
}
