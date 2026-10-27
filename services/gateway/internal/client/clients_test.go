package client

import (
	"context"
	"net"
	"testing"
	"time"

	authpb "github.com/deaprima/linkforge/services/shared/proto/auth"
	linkpb "github.com/deaprima/linkforge/services/shared/proto/link"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"
)

// authStub hanya mengimplementasi Login; RPC lain jatuh ke Unimplemented.
type authStub struct {
	authpb.UnimplementedAuthServiceServer
}

func (authStub) Login(_ context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	return &authpb.LoginResponse{User: &authpb.User{Email: req.GetEmail()}}, nil
}

type linkStub struct {
	linkpb.UnimplementedLinkServiceServer
}

func (linkStub) ResolveAlias(_ context.Context, req *linkpb.ResolveAliasRequest) (*linkpb.ResolveAliasResponse, error) {
	return &linkpb.ResolveAliasResponse{OriginalUrl: "https://example.com/" + req.GetAlias()}, nil
}

func startServer(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	authpb.RegisterAuthServiceServer(srv, authStub{})
	linkpb.RegisterLinkServiceServer(srv, linkStub{})
	healthpb.RegisterHealthServer(srv, health.NewServer())
	go srv.Serve(lis)
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func TestAuthClient_AgainstRealServer(t *testing.T) {
	addr := startServer(t)
	c, err := NewAuthClient(addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()

	if err := c.Check(ctx); err != nil {
		t.Fatalf("health check: %v", err)
	}

	res, err := c.Login(ctx, &authpb.LoginRequest{Email: "dea@example.com"})
	if err != nil || res.GetUser().GetEmail() != "dea@example.com" {
		t.Fatalf("Login routed incorrectly: %v %v", res, err)
	}

	// Sisanya belum diimplementasi di stub → Unimplemented diteruskan apa adanya
	calls := map[string]func() error{
		"Register":       func() error { _, err := c.Register(ctx, &authpb.RegisterRequest{}); return err },
		"RefreshToken":   func() error { _, err := c.RefreshToken(ctx, &authpb.RefreshTokenRequest{}); return err },
		"Logout":         func() error { _, err := c.Logout(ctx, &authpb.LogoutRequest{}); return err },
		"ValidateToken":  func() error { _, err := c.ValidateToken(ctx, &authpb.ValidateTokenRequest{}); return err },
		"GetUser":        func() error { _, err := c.GetUser(ctx, &authpb.GetUserRequest{}); return err },
		"CreateApiKey":   func() error { _, err := c.CreateApiKey(ctx, &authpb.CreateApiKeyRequest{}); return err },
		"ListApiKeys":    func() error { _, err := c.ListApiKeys(ctx, &authpb.ListApiKeysRequest{}); return err },
		"DeleteApiKey":   func() error { _, err := c.DeleteApiKey(ctx, &authpb.DeleteApiKeyRequest{}); return err },
		"ValidateApiKey": func() error { _, err := c.ValidateApiKey(ctx, &authpb.ValidateApiKeyRequest{}); return err },
		"GoogleAuth":     func() error { _, err := c.GoogleAuth(ctx, &authpb.GoogleAuthRequest{}); return err },
	}
	for name, fn := range calls {
		if code := status.Code(fn()); code != codes.Unimplemented {
			t.Errorf("%s: code = %s, want Unimplemented", name, code)
		}
	}
}

func TestLinkClient_AgainstRealServer(t *testing.T) {
	addr := startServer(t)
	c, err := NewLinkClient(addr, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()

	if err := c.Check(ctx); err != nil {
		t.Fatalf("health check: %v", err)
	}

	res, err := c.ResolveAlias(ctx, &linkpb.ResolveAliasRequest{Alias: "abc"})
	if err != nil || res.GetOriginalUrl() != "https://example.com/abc" {
		t.Fatalf("ResolveAlias routed incorrectly: %v %v", res, err)
	}

	calls := map[string]func() error{
		"CreateLink":         func() error { _, err := c.CreateLink(ctx, &linkpb.CreateLinkRequest{}); return err },
		"GetLink":            func() error { _, err := c.GetLink(ctx, &linkpb.GetLinkRequest{}); return err },
		"ListLinks":          func() error { _, err := c.ListLinks(ctx, &linkpb.ListLinksRequest{}); return err },
		"UpdateLink":         func() error { _, err := c.UpdateLink(ctx, &linkpb.UpdateLinkRequest{}); return err },
		"DeleteLink":         func() error { _, err := c.DeleteLink(ctx, &linkpb.DeleteLinkRequest{}); return err },
		"VerifyLinkPassword": func() error { _, err := c.VerifyLinkPassword(ctx, &linkpb.VerifyLinkPasswordRequest{}); return err },
		"GetUserLinkIDs":     func() error { _, err := c.GetUserLinkIDs(ctx, &linkpb.GetUserLinkIDsRequest{}); return err },
	}
	for name, fn := range calls {
		if code := status.Code(fn()); code != codes.Unimplemented {
			t.Errorf("%s: code = %s, want Unimplemented", name, code)
		}
	}
}
