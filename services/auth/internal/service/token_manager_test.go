package service

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func TestTokenManager_RoundTrip(t *testing.T) {
	tm := NewTokenManager(testSecret, time.Minute)
	userID := uuid.New()

	token, err := tm.GenerateAccessToken(userID, "dea@example.com")
	if err != nil {
		t.Fatal(err)
	}
	claims, err := tm.ValidateAccessToken(token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.UserID != userID.String() || claims.Email != "dea@example.com" {
		t.Errorf("claims = %+v", claims)
	}
	if exp := claims.ExpiresAt.Time; exp.Before(time.Now()) || exp.After(time.Now().Add(time.Minute+time.Second)) {
		t.Errorf("expiry %v not within access duration", exp)
	}
}

func TestTokenManager_UsesHS256(t *testing.T) {
	tm := NewTokenManager(testSecret, time.Minute)
	token, _ := tm.GenerateAccessToken(uuid.New(), "a@b.com")

	parsed, _, err := jwt.NewParser().ParseUnverified(token, &UserClaims{})
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Method.Alg() != "HS256" {
		t.Errorf("alg = %s, want HS256", parsed.Method.Alg())
	}
}

func TestTokenManager_Rejects(t *testing.T) {
	tm := NewTokenManager(testSecret, time.Minute)
	valid, _ := tm.GenerateAccessToken(uuid.New(), "a@b.com")

	expired, _ := NewTokenManager(testSecret, -time.Minute).GenerateAccessToken(uuid.New(), "a@b.com")
	otherSecret, _ := NewTokenManager("other-secret", time.Minute).GenerateAccessToken(uuid.New(), "a@b.com")
	unsigned, _ := jwt.NewWithClaims(jwt.SigningMethodNone, UserClaims{UserID: "x"}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	tests := map[string]string{
		"expired token":            expired,
		"signed with other secret": otherSecret,
		"alg none":                 unsigned,
		"tampered payload":         valid[:len(valid)-2] + "xx",
		"garbage":                  "not.a.jwt",
		"empty":                    "",
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := tm.ValidateAccessToken(token); err == nil {
				t.Error("token must be rejected")
			}
		})
	}
}

func TestTokenManager_RefreshTokenIsRandom(t *testing.T) {
	tm := NewTokenManager(testSecret, time.Minute)
	a, _ := tm.GenerateRefreshToken()
	b, _ := tm.GenerateRefreshToken()
	if len(a) != 64 || a == b {
		t.Errorf("refresh tokens must be 64 hex chars and unique: %q %q", a, b)
	}
}
