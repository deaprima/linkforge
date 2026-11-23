// Package metrics berisi instrumentasi Prometheus yang dipakai bersama oleh semua service.
package metrics

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var latencyBuckets = []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5}

var (
	grpcServerHandled = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_server_handled_total",
		Help: "gRPC calls yang selesai ditangani server, per method dan status code.",
	}, []string{"grpc_method", "grpc_code"})

	grpcServerLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_server_handling_seconds",
		Help:    "Durasi penanganan gRPC call di sisi server (unary saja).",
		Buckets: latencyBuckets,
	}, []string{"grpc_method"})

	grpcClientHandled = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_client_handled_total",
		Help: "gRPC calls keluar per dependency dan status code.",
	}, []string{"dependency", "grpc_method", "grpc_code"})

	grpcClientLatency = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_client_handling_seconds",
		Help:    "Durasi gRPC call keluar per dependency.",
		Buckets: latencyBuckets,
	}, []string{"dependency", "grpc_method"})
)

// shortMethod: "/linkforge.auth.AuthService/Login" -> "AuthService/Login".
func shortMethod(full string) string {
	full = strings.TrimPrefix(full, "/")
	if i := strings.LastIndex(full, "."); i >= 0 {
		return full[i+1:]
	}
	return full
}

// UnaryServerInterceptor mencatat jumlah dan latensi setiap unary call.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		m := shortMethod(info.FullMethod)
		grpcServerLatency.WithLabelValues(m).Observe(time.Since(start).Seconds())
		grpcServerHandled.WithLabelValues(m, status.Code(err).String()).Inc()
		return resp, err
	}
}

// StreamServerInterceptor hanya menghitung call yang selesai (stream long-lived tidak cocok untuk histogram latensi).
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := handler(srv, ss)
		grpcServerHandled.WithLabelValues(shortMethod(info.FullMethod), status.Code(err).String()).Inc()
		return err
	}
}

// UnaryClientInterceptor mencatat latensi dan hasil call ke satu dependency (mis. "auth", "link").
func UnaryClientInterceptor(dependency string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		start := time.Now()
		err := invoker(ctx, method, req, reply, cc, opts...)
		m := shortMethod(method)
		grpcClientLatency.WithLabelValues(dependency, m).Observe(time.Since(start).Seconds())
		grpcClientHandled.WithLabelValues(dependency, m, status.Code(err).String()).Inc()
		return err
	}
}
