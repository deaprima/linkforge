package repository

import (
	"context"
	"testing"
	"time"

	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/deaprima/linkforge/services/shared/testdb"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Integration test terhadap Postgres sungguhan; di-skip kalau TEST_DSN tidak di-set (lihat shared/testdb).

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testdb.Open(t, "auth",
		&entity.User{}, &entity.OAuthAccount{}, &entity.RefreshToken{}, &entity.ApiKey{}, &entity.PasswordResetToken{})
}

func seedUser(t *testing.T, db *gorm.DB, email string) *entity.User {
	t.Helper()
	hash := "hash-" + email
	u := &entity.User{Email: email, Name: "Test", PasswordHash: &hash}
	if err := NewUserRepository(db).Create(context.Background(), u); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestUserRepository(t *testing.T) {
	db := newDB(t)
	repo := NewUserRepository(db)
	ctx := context.Background()
	u := seedUser(t, db, "dea@example.com")

	t.Run("get by id and email", func(t *testing.T) {
		byID, err := repo.GetByID(ctx, u.ID)
		if err != nil || byID == nil || byID.Email != "dea@example.com" {
			t.Fatalf("GetByID = %+v, %v", byID, err)
		}
		byEmail, err := repo.GetByEmail(ctx, "dea@example.com")
		if err != nil || byEmail == nil || byEmail.ID != u.ID {
			t.Fatalf("GetByEmail = %+v, %v", byEmail, err)
		}
	})

	t.Run("missing user is nil, not an error", func(t *testing.T) {
		if got, err := repo.GetByID(ctx, uuid.New()); got != nil || err != nil {
			t.Errorf("GetByID(unknown) = %v, %v", got, err)
		}
		if got, err := repo.GetByEmail(ctx, "nobody@example.com"); got != nil || err != nil {
			t.Errorf("GetByEmail(unknown) = %v, %v", got, err)
		}
	})

	t.Run("duplicate email is rejected", func(t *testing.T) {
		dup := &entity.User{Email: "dea@example.com", Name: "Other"}
		if err := repo.Create(ctx, dup); err == nil {
			t.Error("want unique violation on duplicate email")
		}
	})

	t.Run("update password", func(t *testing.T) {
		if err := repo.UpdatePassword(ctx, u.ID, "new-hash"); err != nil {
			t.Fatal(err)
		}
		got, _ := repo.GetByID(ctx, u.ID)
		if got.PasswordHash == nil || *got.PasswordHash != "new-hash" {
			t.Errorf("password hash = %v", got.PasswordHash)
		}
	})
}

func TestTokenRepository(t *testing.T) {
	db := newDB(t)
	repo := NewTokenRepository(db)
	ctx := context.Background()
	u := seedUser(t, db, "tok@example.com")
	other := seedUser(t, db, "other@example.com")

	mk := func(userID uuid.UUID, hash string) *entity.RefreshToken {
		tok := &entity.RefreshToken{UserID: userID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)}
		if err := repo.Create(ctx, tok); err != nil {
			t.Fatal(err)
		}
		return tok
	}
	a, b, c := mk(u.ID, "hash-a"), mk(u.ID, "hash-b"), mk(other.ID, "hash-c")

	got, err := repo.GetByHash(ctx, "hash-a")
	if err != nil || got == nil || got.ID != a.ID || got.RevokedAt != nil {
		t.Fatalf("GetByHash = %+v, %v", got, err)
	}
	if got, err := repo.GetByHash(ctx, "missing"); got != nil || err != nil {
		t.Errorf("GetByHash(missing) = %v, %v", got, err)
	}

	if err := repo.Revoke(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetByHash(ctx, "hash-a"); got.RevokedAt == nil {
		t.Error("Revoke must set revoked_at")
	}

	// Reuse detection: cabut semua token milik satu user, tidak menyentuh user lain
	if err := repo.RevokeAllByUserID(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetByHash(ctx, "hash-b"); got.RevokedAt == nil {
		t.Errorf("token %s of the same user must be revoked", b.ID)
	}
	if got, _ := repo.GetByHash(ctx, "hash-c"); got.RevokedAt != nil {
		t.Errorf("token %s of another user must stay valid", c.ID)
	}
}

func TestApiKeyRepository(t *testing.T) {
	db := newDB(t)
	repo := NewApiKeyRepository(db)
	ctx := context.Background()
	u := seedUser(t, db, "key@example.com")
	other := seedUser(t, db, "otherkey@example.com")

	mk := func(userID uuid.UUID, name, hash string) *entity.ApiKey {
		k := &entity.ApiKey{UserID: userID, Name: name, KeyHash: hash, KeyPrefix: "lf_sk_ab"}
		if err := repo.Create(ctx, k); err != nil {
			t.Fatal(err)
		}
		return k
	}
	first := mk(u.ID, "first", "kh-1")
	time.Sleep(10 * time.Millisecond) // created_at berbeda supaya urutan DESC pasti
	second := mk(u.ID, "second", "kh-2")
	mk(other.ID, "foreign", "kh-3")

	t.Run("lookup by id and hash", func(t *testing.T) {
		byID, err := repo.GetByID(ctx, first.ID)
		if err != nil || byID == nil || byID.Name != "first" {
			t.Fatalf("GetByID = %+v, %v", byID, err)
		}
		byHash, err := repo.GetByHash(ctx, "kh-2")
		if err != nil || byHash == nil || byHash.ID != second.ID {
			t.Fatalf("GetByHash = %+v, %v", byHash, err)
		}
		if got, err := repo.GetByID(ctx, uuid.New()); got != nil || err != nil {
			t.Errorf("GetByID(unknown) = %v, %v", got, err)
		}
		if got, err := repo.GetByHash(ctx, "nope"); got != nil || err != nil {
			t.Errorf("GetByHash(unknown) = %v, %v", got, err)
		}
	})

	t.Run("list is per user, newest first", func(t *testing.T) {
		keys, err := repo.ListByUserID(ctx, u.ID)
		if err != nil || len(keys) != 2 || keys[0].ID != second.ID || keys[1].ID != first.ID {
			t.Fatalf("ListByUserID = %+v, %v", keys, err)
		}
	})

	t.Run("update last used", func(t *testing.T) {
		if err := repo.UpdateLastUsed(ctx, first.ID); err != nil {
			t.Fatal(err)
		}
		got, _ := repo.GetByID(ctx, first.ID)
		if got.LastUsedAt == nil {
			t.Error("last_used_at not set")
		}
	})

	t.Run("revoke only works for the owner", func(t *testing.T) {
		if err := repo.Revoke(ctx, first.ID, other.ID); err == nil {
			t.Error("another user must not be able to revoke the key")
		}
		if err := repo.Revoke(ctx, first.ID, u.ID); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.GetByHash(ctx, "kh-1"); got != nil {
			t.Error("revoked key must not be found by hash")
		}
		keys, _ := repo.ListByUserID(ctx, u.ID)
		if len(keys) != 1 || keys[0].ID != second.ID {
			t.Errorf("revoked key must disappear from the list: %+v", keys)
		}
		if err := repo.Revoke(ctx, first.ID, u.ID); err == nil {
			t.Error("revoking twice must report not found")
		}
	})
}

func TestOAuthRepository(t *testing.T) {
	db := newDB(t)
	repo := NewOAuthRepository(db)
	ctx := context.Background()
	u := seedUser(t, db, "oauth@example.com")

	acc := &entity.OAuthAccount{UserID: u.ID, Provider: "google", ProviderUserID: "g-123", Email: "oauth@example.com"}
	if err := repo.Create(ctx, acc); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByProviderAndUserID(ctx, "google", "g-123")
	if err != nil || got == nil || got.UserID != u.ID {
		t.Fatalf("GetByProviderAndUserID = %+v, %v", got, err)
	}
	if got, err := repo.GetByProviderAndUserID(ctx, "google", "other"); got != nil || err != nil {
		t.Errorf("unknown account = %v, %v", got, err)
	}
	if got, _ := repo.GetByProviderAndUserID(ctx, "github", "g-123"); got != nil {
		t.Error("provider must be part of the lookup")
	}

	dup := &entity.OAuthAccount{UserID: u.ID, Provider: "google", ProviderUserID: "g-123", Email: "oauth@example.com"}
	if err := repo.Create(ctx, dup); err == nil {
		t.Error("same provider account must not be linked twice")
	}
}

func TestPasswordResetRepository(t *testing.T) {
	db := newDB(t)
	repo := NewPasswordResetRepository(db)
	ctx := context.Background()
	u := seedUser(t, db, "reset@example.com")
	other := seedUser(t, db, "otherreset@example.com")

	mk := func(userID uuid.UUID, hash string) *entity.PasswordResetToken {
		tok := &entity.PasswordResetToken{UserID: userID, TokenHash: hash, ExpiresAt: time.Now().Add(time.Hour)}
		if err := repo.Create(ctx, tok); err != nil {
			t.Fatal(err)
		}
		return tok
	}
	a := mk(u.ID, "rt-a")
	mk(u.ID, "rt-b")
	mk(other.ID, "rt-c")

	got, err := repo.GetByHash(ctx, "rt-a")
	if err != nil || got == nil || got.ID != a.ID || got.UsedAt != nil {
		t.Fatalf("GetByHash = %+v, %v", got, err)
	}
	if got, err := repo.GetByHash(ctx, "missing"); got != nil || err != nil {
		t.Errorf("GetByHash(missing) = %v, %v", got, err)
	}

	t.Run("consume is single use", func(t *testing.T) {
		ok, err := repo.Consume(ctx, a.ID)
		if err != nil || !ok {
			t.Fatalf("first consume: %v, %v", ok, err)
		}
		ok, err = repo.Consume(ctx, a.ID)
		if err != nil || ok {
			t.Errorf("second consume must report false, got %v, %v", ok, err)
		}
	})

	t.Run("count since", func(t *testing.T) {
		n, err := repo.CountSince(ctx, u.ID, time.Now().Add(-time.Minute))
		if err != nil || n != 2 {
			t.Errorf("CountSince = %d, %v; want 2", n, err)
		}
		if n, _ := repo.CountSince(ctx, u.ID, time.Now().Add(time.Minute)); n != 0 {
			t.Errorf("CountSince(future) = %d, want 0", n)
		}
	})

	t.Run("invalidate all only touches the user's unused tokens", func(t *testing.T) {
		if err := repo.InvalidateAllByUserID(ctx, u.ID); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.GetByHash(ctx, "rt-b"); got.UsedAt == nil {
			t.Error("rt-b must be invalidated")
		}
		if got, _ := repo.GetByHash(ctx, "rt-c"); got.UsedAt != nil {
			t.Error("another user's token must stay valid")
		}
	})
}
