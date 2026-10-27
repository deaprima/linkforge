package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
)

const (
	testSecret    = "test-secret"
	testClientID  = "client-id.apps.googleusercontent.com"
	testAccessDur = 15 * time.Minute
)

type testEnv struct {
	svc    *authService
	users  *fakeUserRepo
	tokens *fakeTokenRepo
	keys   *fakeKeyRepo
	oauth  *fakeOAuthRepo
	tm     TokenManager
}

func newTestEnv(users ...*entity.User) *testEnv {
	env := &testEnv{
		users:  newFakeUserRepo(users...),
		tokens: newFakeTokenRepo(),
		keys:   newFakeKeyRepo(),
		oauth:  &fakeOAuthRepo{},
		tm:     NewTokenManager(testSecret, testAccessDur),
	}
	env.svc = NewAuthService(env.users, env.tokens, env.keys, env.oauth, testClientID, env.tm, 7*24*time.Hour).(*authService)
	return env
}

func existingUser(t *testing.T, email, password string) *entity.User {
	t.Helper()
	u := &entity.User{ID: uuid.New(), Name: "Existing", Email: email}
	if password != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
		if err != nil {
			t.Fatal(err)
		}
		u.PasswordHash = ptr(string(h))
	}
	return u
}

func wantErrContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), substr) {
		t.Fatalf("want error containing %q, got %v", substr, err)
	}
}

// Register & Login

func TestRegister(t *testing.T) {
	env := newTestEnv()

	user, tokens, err := env.svc.Register(context.Background(), RegisterInput{
		Name: "Dea", Email: "  Dea@Example.com ", Password: "password123",
	})
	if err != nil {
		t.Fatal(err)
	}

	if user.Email != "dea@example.com" {
		t.Errorf("email = %q, want normalized lowercase", user.Email)
	}
	if user.PasswordHash == nil || *user.PasswordHash == "password123" {
		t.Fatal("password must be stored hashed")
	}
	if bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte("password123")) != nil {
		t.Error("stored hash does not match password")
	}

	claims, err := env.tm.ValidateAccessToken(tokens.AccessToken)
	if err != nil || claims.UserID != user.ID.String() || claims.Email != user.Email {
		t.Errorf("access token invalid: claims=%+v err=%v", claims, err)
	}
	stored, _ := env.tokens.GetByHash(context.Background(), hashToken(tokens.RefreshToken))
	if stored == nil || stored.UserID != user.ID {
		t.Error("refresh token must be persisted as a SHA-256 hash for the user")
	}
}

func TestRegister_DuplicateEmailIsCaseInsensitive(t *testing.T) {
	env := newTestEnv(existingUser(t, "dea@example.com", "password123"))

	_, _, err := env.svc.Register(context.Background(), RegisterInput{Name: "X", Email: "DEA@example.com", Password: "password123"})
	wantErrContains(t, err, "email already exists")
}

func TestRegister_RepositoryErrors(t *testing.T) {
	tests := map[string]func(*testEnv){
		"lookup fails":      func(e *testEnv) { e.users.getErr = errDB },
		"create user fails": func(e *testEnv) { e.users.createErr = errDB },
		"save token fails":  func(e *testEnv) { e.tokens.createErr = errDB },
	}
	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			env := newTestEnv()
			setup(env)
			_, _, err := env.svc.Register(context.Background(), RegisterInput{Name: "X", Email: "x@x.com", Password: "password123"})
			if !errors.Is(err, errDB) {
				t.Fatalf("want errDB, got %v", err)
			}
		})
	}
}

func TestLogin(t *testing.T) {
	oauthOnly := &entity.User{ID: uuid.New(), Email: "google@example.com"} // tanpa password

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  string
	}{
		{"valid credentials", "dea@example.com", "password123", ""},
		{"email is case-insensitive", "Dea@Example.COM", "password123", ""},
		{"wrong password", "dea@example.com", "wrong", "invalid credentials"},
		{"unknown email", "nobody@example.com", "password123", "invalid credentials"},
		{"oauth-only account cannot use password login", "google@example.com", "", "invalid credentials"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(existingUser(t, "dea@example.com", "password123"), oauthOnly)
			user, tokens, err := env.svc.Login(context.Background(), LoginInput{Email: tt.email, Password: tt.password})
			if tt.wantErr != "" {
				wantErrContains(t, err, tt.wantErr)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if user.Email != "dea@example.com" || tokens.AccessToken == "" || tokens.RefreshToken == "" {
				t.Errorf("unexpected result: user=%+v tokens=%+v", user, tokens)
			}
		})
	}
}

// Refresh token rotation & reuse detection

func loginTokens(t *testing.T, env *testEnv, email string) *TokenResult {
	t.Helper()
	_, tokens, err := env.svc.Login(context.Background(), LoginInput{Email: email, Password: "password123"})
	if err != nil {
		t.Fatal(err)
	}
	return tokens
}

func TestRefreshToken_RotatesToken(t *testing.T) {
	user := existingUser(t, "dea@example.com", "password123")
	env := newTestEnv(user)
	old := loginTokens(t, env, user.Email)

	fresh, err := env.svc.RefreshToken(context.Background(), old.RefreshToken, "10.0.0.1", "Mozilla/5.0")
	if err != nil {
		t.Fatal(err)
	}
	if fresh.RefreshToken == old.RefreshToken {
		t.Fatal("refresh token must rotate")
	}

	oldEntity, _ := env.tokens.GetByHash(context.Background(), hashToken(old.RefreshToken))
	if oldEntity.RevokedAt == nil {
		t.Error("old refresh token must be revoked after rotation")
	}
	newEntity, _ := env.tokens.GetByHash(context.Background(), hashToken(fresh.RefreshToken))
	if newEntity.IPAddress != "10.0.0.1" || newEntity.UserAgent != "Mozilla/5.0" {
		t.Errorf("client metadata not stored: ip=%q ua=%q", newEntity.IPAddress, newEntity.UserAgent)
	}
	if env.tokens.active(user.ID) != 1 {
		t.Errorf("active tokens = %d, want 1", env.tokens.active(user.ID))
	}
}

func TestRefreshToken_ReuseRevokesAllSessions(t *testing.T) {
	user := existingUser(t, "dea@example.com", "password123")
	env := newTestEnv(user)
	stolen := loginTokens(t, env, user.Email)
	_ = loginTokens(t, env, user.Email) // sesi lain (device lain)

	if _, err := env.svc.RefreshToken(context.Background(), stolen.RefreshToken, "", ""); err != nil {
		t.Fatal(err)
	}

	// Token lama dipakai lagi → indikasi pencurian
	_, err := env.svc.RefreshToken(context.Background(), stolen.RefreshToken, "", "")
	wantErrContains(t, err, "abuse detected")

	if len(env.tokens.revokeAll) != 1 || env.tokens.revokeAll[0] != user.ID {
		t.Errorf("RevokeAllByUserID calls = %v, want [%s]", env.tokens.revokeAll, user.ID)
	}
	if n := env.tokens.active(user.ID); n != 0 {
		t.Errorf("active tokens after reuse = %d, want 0", n)
	}
}

func TestRefreshToken_Rejections(t *testing.T) {
	user := existingUser(t, "dea@example.com", "password123")

	t.Run("unknown token", func(t *testing.T) {
		env := newTestEnv(user)
		_, err := env.svc.RefreshToken(context.Background(), "does-not-exist", "", "")
		wantErrContains(t, err, "invalid refresh token")
	})
	t.Run("expired token", func(t *testing.T) {
		env := newTestEnv(user)
		tokens := loginTokens(t, env, user.Email)
		stored, _ := env.tokens.GetByHash(context.Background(), hashToken(tokens.RefreshToken))
		stored.ExpiresAt = time.Now().Add(-time.Minute)

		_, err := env.svc.RefreshToken(context.Background(), tokens.RefreshToken, "", "")
		wantErrContains(t, err, "expired")
	})
	t.Run("user deleted", func(t *testing.T) {
		env := newTestEnv(user)
		tokens := loginTokens(t, env, user.Email)
		delete(env.users.users, user.ID)

		_, err := env.svc.RefreshToken(context.Background(), tokens.RefreshToken, "", "")
		wantErrContains(t, err, "user not found")
	})
	t.Run("revoke fails", func(t *testing.T) {
		env := newTestEnv(user)
		tokens := loginTokens(t, env, user.Email)
		env.tokens.revokeErr = errDB

		if _, err := env.svc.RefreshToken(context.Background(), tokens.RefreshToken, "", ""); !errors.Is(err, errDB) {
			t.Fatalf("want errDB, got %v", err)
		}
	})
}

func TestLogout(t *testing.T) {
	user := existingUser(t, "dea@example.com", "password123")
	env := newTestEnv(user)
	tokens := loginTokens(t, env, user.Email)

	if err := env.svc.Logout(context.Background(), tokens.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if env.tokens.active(user.ID) != 0 {
		t.Error("refresh token must be revoked after logout")
	}

	err := env.svc.Logout(context.Background(), "unknown-token")
	wantErrContains(t, err, "invalid refresh token")
}

func TestValidateToken(t *testing.T) {
	user := existingUser(t, "dea@example.com", "password123")
	env := newTestEnv(user)
	tokens := loginTokens(t, env, user.Email)

	claims, err := env.svc.ValidateToken(context.Background(), tokens.AccessToken)
	if err != nil || claims.UserID != user.ID.String() {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
	if _, err := env.svc.ValidateToken(context.Background(), "garbage"); err == nil {
		t.Error("garbage token must be rejected")
	}
}

func TestGetUser(t *testing.T) {
	user := existingUser(t, "dea@example.com", "")
	env := newTestEnv(user)

	got, err := env.svc.GetUser(context.Background(), user.ID)
	if err != nil || got.ID != user.ID {
		t.Fatalf("got=%+v err=%v", got, err)
	}

	_, err = env.svc.GetUser(context.Background(), uuid.New())
	wantErrContains(t, err, "user not found")

	env.users.getErr = errDB
	if _, err := env.svc.GetUser(context.Background(), user.ID); !errors.Is(err, errDB) {
		t.Fatalf("want errDB, got %v", err)
	}
}

// API keys

func TestCreateApiKey(t *testing.T) {
	env := newTestEnv()
	userID := uuid.New()

	key, raw, err := env.svc.CreateApiKey(context.Background(), userID, "CLI")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(raw, "lf_sk_") || len(raw) != len("lf_sk_")+64 {
		t.Errorf("raw key %q has unexpected format", raw)
	}
	if key.KeyHash == raw || key.KeyHash != hashToken(raw) {
		t.Error("only the SHA-256 hash of the raw key may be stored")
	}
	if key.KeyPrefix != raw[:12] {
		t.Errorf("prefix = %q, want %q", key.KeyPrefix, raw[:12])
	}

	_, _, err = env.svc.CreateApiKey(context.Background(), userID, "")
	wantErrContains(t, err, "name cannot be empty")

	env.keys.createErr = errDB
	if _, _, err := env.svc.CreateApiKey(context.Background(), userID, "CLI"); !errors.Is(err, errDB) {
		t.Fatalf("want errDB, got %v", err)
	}
}

func TestApiKeyLifecycle(t *testing.T) {
	env := newTestEnv()
	owner := uuid.New()
	ctx := context.Background()

	key, raw, err := env.svc.CreateApiKey(ctx, owner, "CLI")
	if err != nil {
		t.Fatal(err)
	}

	// Validate → mengembalikan key & update last_used_at secara async
	got, err := env.svc.ValidateApiKey(ctx, raw)
	if err != nil || got.UserID != owner {
		t.Fatalf("validate: got=%+v err=%v", got, err)
	}
	select {
	case id := <-env.keys.usedCh:
		if id != key.ID {
			t.Errorf("last_used updated for %s, want %s", id, key.ID)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("last_used_at was not updated")
	}

	if list, _ := env.svc.ListApiKeys(ctx, owner); len(list) != 1 {
		t.Fatalf("list = %d keys, want 1", len(list))
	}

	// User lain tidak bisa revoke key ini
	err = env.svc.DeleteApiKey(ctx, uuid.New(), key.ID)
	wantErrContains(t, err, "not found")

	if err := env.svc.DeleteApiKey(ctx, owner, key.ID); err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.ValidateApiKey(ctx, raw)
	wantErrContains(t, err, "invalid or revoked")

	if list, _ := env.svc.ListApiKeys(ctx, owner); len(list) != 0 {
		t.Errorf("revoked key still listed")
	}
}

func TestValidateApiKey_Errors(t *testing.T) {
	env := newTestEnv()
	_, err := env.svc.ValidateApiKey(context.Background(), "lf_sk_unknown")
	wantErrContains(t, err, "invalid or revoked")

	env.keys.getErr = errDB
	if _, err := env.svc.ValidateApiKey(context.Background(), "lf_sk_unknown"); !errors.Is(err, errDB) {
		t.Fatalf("want errDB, got %v", err)
	}
}

// Google OAuth

func googlePayload(sub, email string, verified bool) *idtoken.Payload {
	return &idtoken.Payload{
		Subject: sub,
		Claims:  map[string]interface{}{"email": email, "name": "Google User", "email_verified": verified},
	}
}

func (env *testEnv) stubGoogle(payload *idtoken.Payload, err error) {
	env.svc.verifyIDToken = func(_ context.Context, _, audience string) (*idtoken.Payload, error) {
		if audience != testClientID {
			return nil, errors.New("wrong audience")
		}
		return payload, err
	}
}

func TestGoogleAuth_NewUser(t *testing.T) {
	env := newTestEnv()
	env.stubGoogle(googlePayload("google-sub-1", "New@Gmail.com", true), nil)

	user, tokens, err := env.svc.GoogleAuth(context.Background(), "id-token")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "new@gmail.com" || user.PasswordHash != nil {
		t.Errorf("unexpected user: %+v", user)
	}
	if len(env.oauth.accounts) != 1 || env.oauth.accounts[0].UserID != user.ID {
		t.Error("oauth account must be linked to the new user")
	}
	if tokens.AccessToken == "" {
		t.Error("tokens must be issued")
	}
}

func TestGoogleAuth_LinksExistingEmailAccount(t *testing.T) {
	existing := existingUser(t, "dea@example.com", "password123")
	env := newTestEnv(existing)
	env.stubGoogle(googlePayload("google-sub-1", "Dea@Example.com", true), nil)

	user, _, err := env.svc.GoogleAuth(context.Background(), "id-token")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != existing.ID {
		t.Errorf("logged in as %s, want existing user %s", user.ID, existing.ID)
	}
	if len(env.users.users) != 1 {
		t.Error("no duplicate user may be created")
	}
}

func TestGoogleAuth_ReturningUser(t *testing.T) {
	existing := existingUser(t, "dea@example.com", "")
	env := newTestEnv(existing)
	env.oauth.accounts = []*entity.OAuthAccount{{ID: uuid.New(), UserID: existing.ID, Provider: "google", ProviderUserID: "google-sub-1"}}
	env.stubGoogle(googlePayload("google-sub-1", "dea@example.com", true), nil)

	user, _, err := env.svc.GoogleAuth(context.Background(), "id-token")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID != existing.ID || len(env.oauth.accounts) != 1 {
		t.Errorf("returning user must reuse the linked account")
	}
}

func TestGoogleAuth_Rejections(t *testing.T) {
	victim := existingUser(t, "victim@company.com", "password123")

	tests := []struct {
		name    string
		payload *idtoken.Payload
		err     error
		wantErr string
	}{
		{"invalid id token", nil, errors.New("token expired"), "invalid google id token"},
		{"no email claim", googlePayload("sub", "", true), nil, "does not provide email"},
		{"unverified email cannot take over existing account", googlePayload("attacker-sub", "victim@company.com", false), nil, "not verified"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(victim)
			env.stubGoogle(tt.payload, tt.err)

			_, _, err := env.svc.GoogleAuth(context.Background(), "id-token")
			wantErrContains(t, err, tt.wantErr)
			if len(env.oauth.accounts) != 0 {
				t.Error("no oauth account may be linked on rejection")
			}
		})
	}
}

func TestGoogleAuth_LinkedUserMissing(t *testing.T) {
	env := newTestEnv()
	env.oauth.accounts = []*entity.OAuthAccount{{UserID: uuid.New(), Provider: "google", ProviderUserID: "sub"}}
	env.stubGoogle(googlePayload("sub", "a@gmail.com", true), nil)

	_, _, err := env.svc.GoogleAuth(context.Background(), "id-token")
	wantErrContains(t, err, "linked user not found")
}
