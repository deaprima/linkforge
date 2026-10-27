package grpc

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/deaprima/linkforge/services/auth/internal/service"
	pb "github.com/deaprima/linkforge/services/shared/proto/auth"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// fakeAuthService mengembalikan nilai yang sudah dikonfigurasi dan mencatat input penting.
type fakeAuthService struct {
	user   *entity.User
	tokens *service.TokenResult
	claims *service.UserClaims
	key    *entity.ApiKey
	keys   []entity.ApiKey
	rawKey string
	err    error

	gotIP, gotUA string
	gotUserID    uuid.UUID
	gotKeyID     uuid.UUID
}

func (f *fakeAuthService) Register(context.Context, service.RegisterInput) (*entity.User, *service.TokenResult, error) {
	return f.user, f.tokens, f.err
}
func (f *fakeAuthService) Login(context.Context, service.LoginInput) (*entity.User, *service.TokenResult, error) {
	return f.user, f.tokens, f.err
}
func (f *fakeAuthService) ValidateToken(context.Context, string) (*service.UserClaims, error) {
	return f.claims, f.err
}
func (f *fakeAuthService) RefreshToken(_ context.Context, _ string, ip, ua string) (*service.TokenResult, error) {
	f.gotIP, f.gotUA = ip, ua
	return f.tokens, f.err
}
func (f *fakeAuthService) Logout(context.Context, string) error { return f.err }
func (f *fakeAuthService) GetUser(_ context.Context, id uuid.UUID) (*entity.User, error) {
	f.gotUserID = id
	return f.user, f.err
}
func (f *fakeAuthService) CreateApiKey(_ context.Context, id uuid.UUID, _ string) (*entity.ApiKey, string, error) {
	f.gotUserID = id
	return f.key, f.rawKey, f.err
}
func (f *fakeAuthService) ListApiKeys(context.Context, uuid.UUID) ([]entity.ApiKey, error) {
	return f.keys, f.err
}
func (f *fakeAuthService) DeleteApiKey(_ context.Context, userID, keyID uuid.UUID) error {
	f.gotUserID, f.gotKeyID = userID, keyID
	return f.err
}
func (f *fakeAuthService) ValidateApiKey(context.Context, string) (*entity.ApiKey, error) {
	return f.key, f.err
}
func (f *fakeAuthService) GoogleAuth(context.Context, string) (*entity.User, *service.TokenResult, error) {
	return f.user, f.tokens, f.err
}

var (
	testUser   = &entity.User{ID: uuid.New(), Name: "Dea", Email: "dea@example.com", CreatedAt: time.Date(2026, 7, 8, 7, 0, 0, 0, time.UTC)}
	testTokens = &service.TokenResult{AccessToken: "access", RefreshToken: "refresh"}
)

func wantCode(t *testing.T, err error, want codes.Code) {
	t.Helper()
	if got := status.Code(err); got != want {
		t.Fatalf("code = %s, want %s (err %v)", got, want, err)
	}
}

func TestRegisterAndLogin(t *testing.T) {
	tests := []struct {
		name string
		call func(h *AuthHandler) (*pb.User, *pb.TokenPair, error)
		err  error
		want codes.Code
	}{
		{"register ok", register, nil, codes.OK},
		{"register duplicate email", register, errors.New("email already exists"), codes.AlreadyExists},
		{"register db error", register, errors.New("db down"), codes.Internal},
		{"login ok", login, nil, codes.OK},
		{"login wrong credentials", login, errors.New("invalid credentials"), codes.Unauthenticated},
		{"login db error", login, errors.New("db down"), codes.Internal},
		{"google ok", google, nil, codes.OK},
		{"google invalid token", google, errors.New("invalid google id token: email is not verified"), codes.Unauthenticated},
		{"google other error", google, errors.New("failed to link google account"), codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewAuthHandler(&fakeAuthService{user: testUser, tokens: testTokens, err: tt.err})
			user, tokens, err := tt.call(h)
			wantCode(t, err, tt.want)
			if tt.want == codes.OK && (user.GetId() != testUser.ID.String() || user.GetEmail() != testUser.Email || tokens.GetAccessToken() != "access" || tokens.GetRefreshToken() != "refresh") {
				t.Errorf("user=%v tokens=%v", user, tokens)
			}
		})
	}
}

func register(h *AuthHandler) (*pb.User, *pb.TokenPair, error) {
	res, err := h.Register(context.Background(), &pb.RegisterRequest{})
	return res.GetUser(), res.GetTokens(), err
}
func login(h *AuthHandler) (*pb.User, *pb.TokenPair, error) {
	res, err := h.Login(context.Background(), &pb.LoginRequest{})
	return res.GetUser(), res.GetTokens(), err
}
func google(h *AuthHandler) (*pb.User, *pb.TokenPair, error) {
	res, err := h.GoogleAuth(context.Background(), &pb.GoogleAuthRequest{})
	return res.GetUser(), res.GetTokens(), err
}

func TestValidateToken(t *testing.T) {
	h := NewAuthHandler(&fakeAuthService{claims: &service.UserClaims{UserID: "user-1", Email: "a@b.com"}})
	res, err := h.ValidateToken(context.Background(), &pb.ValidateTokenRequest{})
	if err != nil || !res.IsValid || res.UserId != "user-1" || res.Email != "a@b.com" {
		t.Errorf("valid: %v %v", res, err)
	}

	// Token tidak valid = respons normal is_valid=false, bukan error gRPC (supaya breaker tidak terpengaruh)
	h = NewAuthHandler(&fakeAuthService{err: errors.New("token is expired")})
	res, err = h.ValidateToken(context.Background(), &pb.ValidateTokenRequest{})
	if err != nil || res.IsValid {
		t.Errorf("invalid: %v %v", res, err)
	}
}

func TestRefreshToken(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"ok", nil, codes.OK},
		{"reuse detected", errors.New("refresh token abuse detected; all sessions revoked"), codes.PermissionDenied},
		{"unknown token", errors.New("invalid refresh token"), codes.Unauthenticated},
		{"expired token", errors.New("refresh token expired"), codes.Unauthenticated},
		{"user gone", errors.New("user not found"), codes.Unauthenticated},
		{"db down is not a 401", errors.New("connection refused"), codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewAuthHandler(&fakeAuthService{tokens: testTokens, err: tt.err})
			res, err := h.RefreshToken(context.Background(), &pb.RefreshTokenRequest{})
			wantCode(t, err, tt.want)
			if tt.want == codes.OK && res.GetTokens().GetRefreshToken() != "refresh" {
				t.Errorf("tokens = %v", res.GetTokens())
			}
		})
	}
}

func TestRefreshToken_ClientMetadata(t *testing.T) {
	tests := []struct {
		name   string
		md     metadata.MD
		wantIP string
		wantUA string
	}{
		{"gateway forwarded ua wins", metadata.Pairs("x-forwarded-for", "1.2.3.4", "x-user-agent", "Mozilla/5.0", "user-agent", "grpc-go/1.0"), "1.2.3.4", "Mozilla/5.0"},
		{"falls back to transport ua", metadata.Pairs("user-agent", "grpc-go/1.0"), "", "grpc-go/1.0"},
		{"no metadata", nil, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &fakeAuthService{tokens: testTokens}
			ctx := context.Background()
			if tt.md != nil {
				ctx = metadata.NewIncomingContext(ctx, tt.md)
			}
			if _, err := NewAuthHandler(svc).RefreshToken(ctx, &pb.RefreshTokenRequest{}); err != nil {
				t.Fatal(err)
			}
			if svc.gotIP != tt.wantIP || svc.gotUA != tt.wantUA {
				t.Errorf("ip=%q ua=%q, want ip=%q ua=%q", svc.gotIP, svc.gotUA, tt.wantIP, tt.wantUA)
			}
		})
	}
}

func TestLogout(t *testing.T) {
	tests := []struct {
		err  error
		want codes.Code
	}{
		{nil, codes.OK},
		{errors.New("invalid refresh token"), codes.InvalidArgument},
		{errors.New("db down"), codes.Internal},
	}
	for _, tt := range tests {
		res, err := NewAuthHandler(&fakeAuthService{err: tt.err}).Logout(context.Background(), &pb.LogoutRequest{})
		wantCode(t, err, tt.want)
		if tt.want == codes.OK && !res.Success {
			t.Error("success must be true")
		}
	}
}

func TestGetUser(t *testing.T) {
	tests := []struct {
		name   string
		userID string
		err    error
		want   codes.Code
	}{
		{"ok", testUser.ID.String(), nil, codes.OK},
		{"invalid uuid", "not-a-uuid", nil, codes.InvalidArgument},
		{"not found", testUser.ID.String(), errors.New("user not found"), codes.NotFound},
		{"db error", testUser.ID.String(), errors.New("db down"), codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := NewAuthHandler(&fakeAuthService{user: testUser, err: tt.err}).GetUser(context.Background(), &pb.GetUserRequest{UserId: tt.userID})
			wantCode(t, err, tt.want)
			if tt.want == codes.OK && !res.GetUser().GetCreatedAt().AsTime().Equal(testUser.CreatedAt) {
				t.Errorf("created_at = %v, want %v", res.GetUser().GetCreatedAt(), testUser.CreatedAt)
			}
		})
	}
}

func TestApiKeys(t *testing.T) {
	used := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	key := &entity.ApiKey{ID: uuid.New(), Name: "CLI", CreatedAt: time.Now()}
	userID := uuid.New().String()

	t.Run("create", func(t *testing.T) {
		res, err := NewAuthHandler(&fakeAuthService{key: key, rawKey: "lf_sk_raw"}).CreateApiKey(context.Background(), &pb.CreateApiKeyRequest{UserId: userID, Name: "CLI"})
		if err != nil || res.RawKey != "lf_sk_raw" || res.ApiKey.Id != key.ID.String() || res.ApiKey.LastUsedAt != nil {
			t.Errorf("res=%v err=%v", res, err)
		}
		_, err = NewAuthHandler(&fakeAuthService{}).CreateApiKey(context.Background(), &pb.CreateApiKeyRequest{UserId: "bad"})
		wantCode(t, err, codes.InvalidArgument)
		_, err = NewAuthHandler(&fakeAuthService{err: errDB}).CreateApiKey(context.Background(), &pb.CreateApiKeyRequest{UserId: userID})
		wantCode(t, err, codes.Internal)
	})

	t.Run("list", func(t *testing.T) {
		keys := []entity.ApiKey{{ID: uuid.New(), Name: "a", LastUsedAt: &used}, {ID: uuid.New(), Name: "b"}}
		res, err := NewAuthHandler(&fakeAuthService{keys: keys}).ListApiKeys(context.Background(), &pb.ListApiKeysRequest{UserId: userID})
		if err != nil || len(res.ApiKeys) != 2 {
			t.Fatalf("res=%v err=%v", res, err)
		}
		if !res.ApiKeys[0].LastUsedAt.AsTime().Equal(used) || res.ApiKeys[1].LastUsedAt != nil {
			t.Errorf("last_used_at not converted correctly: %v", res.ApiKeys)
		}
		_, err = NewAuthHandler(&fakeAuthService{}).ListApiKeys(context.Background(), &pb.ListApiKeysRequest{UserId: "bad"})
		wantCode(t, err, codes.InvalidArgument)
		_, err = NewAuthHandler(&fakeAuthService{err: errDB}).ListApiKeys(context.Background(), &pb.ListApiKeysRequest{UserId: userID})
		wantCode(t, err, codes.Internal)
	})

	t.Run("delete", func(t *testing.T) {
		keyID := uuid.New()
		svc := &fakeAuthService{}
		if _, err := NewAuthHandler(svc).DeleteApiKey(context.Background(), &pb.DeleteApiKeyRequest{UserId: userID, ApiKeyId: keyID.String()}); err != nil {
			t.Fatal(err)
		}
		if svc.gotKeyID != keyID || svc.gotUserID.String() != userID {
			t.Errorf("service got user=%s key=%s", svc.gotUserID, svc.gotKeyID)
		}

		cases := []struct {
			user, key string
			err       error
			want      codes.Code
		}{
			{"bad", keyID.String(), nil, codes.InvalidArgument},
			{userID, "bad", nil, codes.InvalidArgument},
			{userID, keyID.String(), errors.New("api key not found"), codes.NotFound},
			{userID, keyID.String(), errDB, codes.Internal},
		}
		for _, c := range cases {
			_, err := NewAuthHandler(&fakeAuthService{err: c.err}).DeleteApiKey(context.Background(), &pb.DeleteApiKeyRequest{UserId: c.user, ApiKeyId: c.key})
			wantCode(t, err, c.want)
		}
	})

	t.Run("validate", func(t *testing.T) {
		owned := &entity.ApiKey{ID: uuid.New(), UserID: uuid.New()}
		res, err := NewAuthHandler(&fakeAuthService{key: owned}).ValidateApiKey(context.Background(), &pb.ValidateApiKeyRequest{})
		if err != nil || !res.IsValid || res.UserId != owned.UserID.String() {
			t.Errorf("valid: %v %v", res, err)
		}
		res, err = NewAuthHandler(&fakeAuthService{err: errors.New("invalid or revoked api key")}).ValidateApiKey(context.Background(), &pb.ValidateApiKeyRequest{})
		if err != nil || res.IsValid {
			t.Errorf("invalid key must return is_valid=false without error: %v %v", res, err)
		}
	})
}

var errDB = errors.New("db down")

func TestListApiKeys_IncludesPrefix(t *testing.T) {
	keys := []entity.ApiKey{{ID: uuid.New(), Name: "CLI", KeyPrefix: "lf_sk_a3f9b2"}}
	res, err := NewAuthHandler(&fakeAuthService{keys: keys}).ListApiKeys(context.Background(), &pb.ListApiKeysRequest{UserId: uuid.NewString()})
	if err != nil || res.ApiKeys[0].KeyPrefix != "lf_sk_a3f9b2" {
		t.Errorf("res=%v err=%v", res, err)
	}
}
