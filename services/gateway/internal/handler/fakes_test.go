package handler_test

import (
	"context"
	"io"

	analyticspb "github.com/deaprima/linkforge/services/shared/proto/analytics"
	authpb "github.com/deaprima/linkforge/services/shared/proto/auth"
	linkpb "github.com/deaprima/linkforge/services/shared/proto/link"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var errUnimplemented = status.Error(codes.Unimplemented, "not stubbed")

// fakeAuth: setiap RPC bisa di-stub lewat field func; yang tidak di-stub mengembalikan Unimplemented.
// Kredensial tetap: "valid-token" / "lf_sk_valid" → user-1.
type fakeAuth struct {
	register     func(context.Context, *authpb.RegisterRequest) (*authpb.RegisterResponse, error)
	login        func(context.Context, *authpb.LoginRequest) (*authpb.LoginResponse, error)
	refreshToken func(context.Context, *authpb.RefreshTokenRequest) (*authpb.RefreshTokenResponse, error)
	logout       func(context.Context, *authpb.LogoutRequest) (*authpb.LogoutResponse, error)
	getUser      func(context.Context, *authpb.GetUserRequest) (*authpb.GetUserResponse, error)
	createApiKey func(context.Context, *authpb.CreateApiKeyRequest) (*authpb.CreateApiKeyResponse, error)
	listApiKeys  func(context.Context, *authpb.ListApiKeysRequest) (*authpb.ListApiKeysResponse, error)
	deleteApiKey func(context.Context, *authpb.DeleteApiKeyRequest) (*authpb.DeleteApiKeyResponse, error)
	googleAuth   func(context.Context, *authpb.GoogleAuthRequest) (*authpb.GoogleAuthResponse, error)
}

func stub[Req, Res any](fn func(context.Context, Req) (Res, error), ctx context.Context, req Req) (Res, error) {
	if fn == nil {
		var zero Res
		return zero, errUnimplemented
	}
	return fn(ctx, req)
}

func (f *fakeAuth) Register(ctx context.Context, r *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	return stub(f.register, ctx, r)
}
func (f *fakeAuth) Login(ctx context.Context, r *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	return stub(f.login, ctx, r)
}
func (f *fakeAuth) RefreshToken(ctx context.Context, r *authpb.RefreshTokenRequest) (*authpb.RefreshTokenResponse, error) {
	return stub(f.refreshToken, ctx, r)
}
func (f *fakeAuth) Logout(ctx context.Context, r *authpb.LogoutRequest) (*authpb.LogoutResponse, error) {
	return stub(f.logout, ctx, r)
}
func (f *fakeAuth) GetUser(ctx context.Context, r *authpb.GetUserRequest) (*authpb.GetUserResponse, error) {
	return stub(f.getUser, ctx, r)
}
func (f *fakeAuth) CreateApiKey(ctx context.Context, r *authpb.CreateApiKeyRequest) (*authpb.CreateApiKeyResponse, error) {
	return stub(f.createApiKey, ctx, r)
}
func (f *fakeAuth) ListApiKeys(ctx context.Context, r *authpb.ListApiKeysRequest) (*authpb.ListApiKeysResponse, error) {
	return stub(f.listApiKeys, ctx, r)
}
func (f *fakeAuth) DeleteApiKey(ctx context.Context, r *authpb.DeleteApiKeyRequest) (*authpb.DeleteApiKeyResponse, error) {
	return stub(f.deleteApiKey, ctx, r)
}
func (f *fakeAuth) GoogleAuth(ctx context.Context, r *authpb.GoogleAuthRequest) (*authpb.GoogleAuthResponse, error) {
	return stub(f.googleAuth, ctx, r)
}

func (f *fakeAuth) ValidateToken(_ context.Context, r *authpb.ValidateTokenRequest) (*authpb.ValidateTokenResponse, error) {
	if r.GetAccessToken() == "valid-token" {
		return &authpb.ValidateTokenResponse{IsValid: true, UserId: "user-1"}, nil
	}
	return &authpb.ValidateTokenResponse{IsValid: false}, nil
}

func (f *fakeAuth) ValidateApiKey(_ context.Context, r *authpb.ValidateApiKeyRequest) (*authpb.ValidateApiKeyResponse, error) {
	if r.GetApiKey() == "lf_sk_valid" {
		return &authpb.ValidateApiKeyResponse{IsValid: true, UserId: "user-1"}, nil
	}
	return &authpb.ValidateApiKeyResponse{IsValid: false}, nil
}

type fakeLink struct {
	createLink         func(context.Context, *linkpb.CreateLinkRequest) (*linkpb.CreateLinkResponse, error)
	getLink            func(context.Context, *linkpb.GetLinkRequest) (*linkpb.GetLinkResponse, error)
	listLinks          func(context.Context, *linkpb.ListLinksRequest) (*linkpb.ListLinksResponse, error)
	updateLink         func(context.Context, *linkpb.UpdateLinkRequest) (*linkpb.UpdateLinkResponse, error)
	deleteLink         func(context.Context, *linkpb.DeleteLinkRequest) (*linkpb.DeleteLinkResponse, error)
	resolveAlias       func(context.Context, *linkpb.ResolveAliasRequest) (*linkpb.ResolveAliasResponse, error)
	verifyLinkPassword func(context.Context, *linkpb.VerifyLinkPasswordRequest) (*linkpb.VerifyLinkPasswordResponse, error)
	getUserLinkIDs     func(context.Context, *linkpb.GetUserLinkIDsRequest) (*linkpb.GetUserLinkIDsResponse, error)
}

func (f *fakeLink) CreateLink(ctx context.Context, r *linkpb.CreateLinkRequest) (*linkpb.CreateLinkResponse, error) {
	return stub(f.createLink, ctx, r)
}
func (f *fakeLink) GetLink(ctx context.Context, r *linkpb.GetLinkRequest) (*linkpb.GetLinkResponse, error) {
	return stub(f.getLink, ctx, r)
}
func (f *fakeLink) ListLinks(ctx context.Context, r *linkpb.ListLinksRequest) (*linkpb.ListLinksResponse, error) {
	return stub(f.listLinks, ctx, r)
}
func (f *fakeLink) UpdateLink(ctx context.Context, r *linkpb.UpdateLinkRequest) (*linkpb.UpdateLinkResponse, error) {
	return stub(f.updateLink, ctx, r)
}
func (f *fakeLink) DeleteLink(ctx context.Context, r *linkpb.DeleteLinkRequest) (*linkpb.DeleteLinkResponse, error) {
	return stub(f.deleteLink, ctx, r)
}
func (f *fakeLink) ResolveAlias(ctx context.Context, r *linkpb.ResolveAliasRequest) (*linkpb.ResolveAliasResponse, error) {
	return stub(f.resolveAlias, ctx, r)
}
func (f *fakeLink) VerifyLinkPassword(ctx context.Context, r *linkpb.VerifyLinkPasswordRequest) (*linkpb.VerifyLinkPasswordResponse, error) {
	return stub(f.verifyLinkPassword, ctx, r)
}

func (f *fakeLink) GetUserLinkIDs(ctx context.Context, r *linkpb.GetUserLinkIDsRequest) (*linkpb.GetUserLinkIDsResponse, error) {
	return stub(f.getUserLinkIDs, ctx, r)
}

type fakeAnalytics struct {
	getLinkAnalytics  func(context.Context, *analyticspb.GetLinkAnalyticsRequest) (*analyticspb.GetLinkAnalyticsResponse, error)
	getSummary        func(context.Context, *analyticspb.GetSummaryRequest) (*analyticspb.GetSummaryResponse, error)
	subscribeRealtime func(context.Context, *analyticspb.SubscribeRealtimeRequest) (grpc.ServerStreamingClient[analyticspb.ClickEvent], error)
}

func (f *fakeAnalytics) GetLinkAnalytics(ctx context.Context, r *analyticspb.GetLinkAnalyticsRequest) (*analyticspb.GetLinkAnalyticsResponse, error) {
	return stub(f.getLinkAnalytics, ctx, r)
}
func (f *fakeAnalytics) GetSummary(ctx context.Context, r *analyticspb.GetSummaryRequest) (*analyticspb.GetSummaryResponse, error) {
	return stub(f.getSummary, ctx, r)
}
func (f *fakeAnalytics) SubscribeRealtime(ctx context.Context, r *analyticspb.SubscribeRealtimeRequest) (grpc.ServerStreamingClient[analyticspb.ClickEvent], error) {
	return stub(f.subscribeRealtime, ctx, r)
}

// fakeClickStream menyajikan event dari channel; channel ditutup = stream berakhir (Analytics putus).
type fakeClickStream struct {
	grpc.ClientStream // hanya Recv yang dipakai handler
	ctx               context.Context
	events            chan *analyticspb.ClickEvent
}

func (s *fakeClickStream) Recv() (*analyticspb.ClickEvent, error) {
	select {
	case ev, ok := <-s.events:
		if !ok {
			return nil, io.EOF
		}
		return ev, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}
