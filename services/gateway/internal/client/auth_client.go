package client

import (
	"context"
	"fmt"
	"time"

	pb "github.com/deaprima/linkforge/services/shared/proto/auth"
)

type AuthClient struct {
	baseClient
	client pb.AuthServiceClient
}

func NewAuthClient(addr string, timeout time.Duration) (*AuthClient, error) {
	base, err := newBaseClient(addr, "gateway->auth", timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to auth service: %w", err)
	}
	return &AuthClient{
		baseClient: base,
		client:     pb.NewAuthServiceClient(base.conn),
	}, nil
}

// Setiap method dibawah membungkus panggilan gRPC dalam circuit breaker

func (c *AuthClient) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.RegisterResponse, error) {
		return c.client.Register(ctx, req)
	})
}

func (c *AuthClient) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.LoginResponse, error) {
		return c.client.Login(ctx, req)
	})
}

func (c *AuthClient) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.RefreshTokenResponse, error) {
		return c.client.RefreshToken(ctx, req)
	})
}

func (c *AuthClient) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.LogoutResponse, error) {
		return c.client.Logout(ctx, req)
	})
}

func (c *AuthClient) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.ValidateTokenResponse, error) {
		return c.client.ValidateToken(ctx, req)
	})
}

func (c *AuthClient) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.GetUserResponse, error) {
		return c.client.GetUser(ctx, req)
	})
}

func (c *AuthClient) CreateApiKey(ctx context.Context, req *pb.CreateApiKeyRequest) (*pb.CreateApiKeyResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.CreateApiKeyResponse, error) {
		return c.client.CreateApiKey(ctx, req)
	})
}

func (c *AuthClient) ListApiKeys(ctx context.Context, req *pb.ListApiKeysRequest) (*pb.ListApiKeysResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.ListApiKeysResponse, error) {
		return c.client.ListApiKeys(ctx, req)
	})
}

func (c *AuthClient) DeleteApiKey(ctx context.Context, req *pb.DeleteApiKeyRequest) (*pb.DeleteApiKeyResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.DeleteApiKeyResponse, error) {
		return c.client.DeleteApiKey(ctx, req)
	})
}

func (c *AuthClient) ValidateApiKey(ctx context.Context, req *pb.ValidateApiKeyRequest) (*pb.ValidateApiKeyResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.ValidateApiKeyResponse, error) {
		return c.client.ValidateApiKey(ctx, req)
	})
}

func (c *AuthClient) GoogleAuth(ctx context.Context, req *pb.GoogleAuthRequest) (*pb.GoogleAuthResponse, error) {
	return call(ctx, &c.baseClient, func(ctx context.Context) (*pb.GoogleAuthResponse, error) {
		return c.client.GoogleAuth(ctx, req)
	})
}
