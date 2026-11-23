package client

import (
	"context"
	"fmt"
	"time"

	pb "github.com/deaprima/linkforge/services/shared/proto/link"
)

type LinkClient struct {
	baseClient
	client pb.LinkServiceClient
}

func NewLinkClient(addr string, timeout time.Duration) (*LinkClient, error) {
	base, err := newBaseClient(addr, "gateway->link", timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to link service: %w", err)
	}
	return &LinkClient{
		baseClient: base,
		client:     pb.NewLinkServiceClient(base.conn),
	}, nil
}

// Setiap method dibawah membungkus panggilan gRPC dalam circuit breaker

func (c *LinkClient) CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.CreateLinkResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.CreateLinkResponse, error) {
		return c.client.CreateLink(ctx, req)
	})
}

func (c *LinkClient) GetLink(ctx context.Context, req *pb.GetLinkRequest) (*pb.GetLinkResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.GetLinkResponse, error) {
		return c.client.GetLink(ctx, req)
	})
}

func (c *LinkClient) ListLinks(ctx context.Context, req *pb.ListLinksRequest) (*pb.ListLinksResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.ListLinksResponse, error) {
		return c.client.ListLinks(ctx, req)
	})
}

func (c *LinkClient) UpdateLink(ctx context.Context, req *pb.UpdateLinkRequest) (*pb.UpdateLinkResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.UpdateLinkResponse, error) {
		return c.client.UpdateLink(ctx, req)
	})
}

func (c *LinkClient) DeleteLink(ctx context.Context, req *pb.DeleteLinkRequest) (*pb.DeleteLinkResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.DeleteLinkResponse, error) {
		return c.client.DeleteLink(ctx, req)
	})
}

func (c *LinkClient) ResolveAlias(ctx context.Context, req *pb.ResolveAliasRequest) (*pb.ResolveAliasResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.ResolveAliasResponse, error) {
		return c.client.ResolveAlias(ctx, req)
	})
}

func (c *LinkClient) VerifyLinkPassword(ctx context.Context, req *pb.VerifyLinkPasswordRequest) (*pb.VerifyLinkPasswordResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.VerifyLinkPasswordResponse, error) {
		return c.client.VerifyLinkPassword(ctx, req)
	})
}

func (c *LinkClient) GetUserLinkIDs(ctx context.Context, req *pb.GetUserLinkIDsRequest) (*pb.GetUserLinkIDsResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.GetUserLinkIDsResponse, error) {
		return c.client.GetUserLinkIDs(ctx, req)
	})
}
