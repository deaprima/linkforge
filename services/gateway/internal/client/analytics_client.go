package client

import (
	"context"
	"fmt"
	"time"

	pb "github.com/deaprima/linkforge/services/shared/proto/analytics"
	"google.golang.org/grpc"
)

type AnalyticsClient struct {
	baseClient
	client pb.AnalyticsServiceClient
}

func NewAnalyticsClient(addr string, timeout time.Duration) (*AnalyticsClient, error) {
	base, err := newBaseClient(addr, "gateway->analytics", timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to analytics service: %w", err)
	}
	return &AnalyticsClient{
		baseClient: base,
		client:     pb.NewAnalyticsServiceClient(base.conn),
	}, nil
}

// Setiap method dibawah membungkus panggilan gRPC dalam circuit breaker

func (c *AnalyticsClient) GetLinkAnalytics(ctx context.Context, req *pb.GetLinkAnalyticsRequest) (*pb.GetLinkAnalyticsResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.GetLinkAnalyticsResponse, error) {
		return c.client.GetLinkAnalytics(ctx, req)
	})
}

func (c *AnalyticsClient) GetSummary(ctx context.Context, req *pb.GetSummaryRequest) (*pb.GetSummaryResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.GetSummaryResponse, error) {
		return c.client.GetSummary(ctx, req)
	})
}

// SubscribeRealtime membuka stream jangka panjang (umur = koneksi WebSocket), jadi sengaja
// tidak memakai timeout per-call maupun breaker. Stream berhenti saat ctx dibatalkan.
func (c *AnalyticsClient) SubscribeRealtime(ctx context.Context, req *pb.SubscribeRealtimeRequest) (grpc.ServerStreamingClient[pb.ClickEvent], error) {
	return c.client.SubscribeRealtime(ctx, req)
}
