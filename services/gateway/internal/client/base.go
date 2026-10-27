package client

import (
	"context"
	"fmt"
	"time"

	"github.com/deaprima/linkforge/services/shared/breaker"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// baseClient berisi koneksi, health client, dan circuit breaker yang dipakai bersama
// oleh semua client gRPC di Gateway.
type baseClient struct {
	conn    *grpc.ClientConn
	health  healthpb.HealthClient
	breaker breaker.Breaker
	timeout time.Duration
}

func newBaseClient(addr, breakerName string, timeout time.Duration) (baseClient, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return baseClient{}, err
	}
	return baseClient{
		conn:    conn,
		health:  healthpb.NewHealthClient(conn),
		breaker: breaker.NewGoBreaker(breakerName),
		timeout: timeout,
	}, nil
}

// Check memanggil standard gRPC health check (dipakai /readyz).
// Sengaja tidak lewat breaker supaya readiness mencerminkan kondisi asli dependency.
func (b *baseClient) Check(ctx context.Context) error {
	res, err := b.health.Check(ctx, &healthpb.HealthCheckRequest{})
	if err != nil {
		return err
	}
	if res.GetStatus() != healthpb.HealthCheckResponse_SERVING {
		return fmt.Errorf("status %s", res.GetStatus())
	}
	return nil
}

func (b *baseClient) Close() error {
	return b.conn.Close()
}

// isInfraError menentukan apakah error gRPC menandakan dependency bermasalah.
// Hanya error jenis ini yang dihitung sebagai failure oleh circuit breaker —
// error bisnis (NotFound, AlreadyExists, InvalidArgument, dll) tidak boleh membuat breaker trip.
func isInfraError(err error) bool {
	switch status.Code(err) {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Internal,
		codes.Unknown, codes.ResourceExhausted, codes.Aborted:
		return true
	}
	return false
}

// call membungkus satu panggilan gRPC dengan timeout dan circuit breaker.
func call[T any](ctx context.Context, b *baseClient, fn func(context.Context) (T, error)) (T, error) {
	var zero T
	var bizErr error

	res, err := b.breaker.Execute(func() (interface{}, error) {
		ctx, cancel := context.WithTimeout(ctx, b.timeout)
		defer cancel()

		r, err := fn(ctx)
		if err != nil && !isInfraError(err) {
			// Laporkan sukses ke breaker, tapi tetap kembalikan error ke pemanggil
			bizErr = err
			return nil, nil
		}
		return r, err
	})
	if err != nil {
		return zero, err
	}
	if bizErr != nil {
		return zero, bizErr
	}
	return res.(T), nil
}
