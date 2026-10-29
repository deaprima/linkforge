package repository

import (
	"context"
	"testing"
	"time"

	"github.com/deaprima/linkforge/services/link/internal/entity"
	"github.com/deaprima/linkforge/services/shared/testdb"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Integration test terhadap Postgres sungguhan; di-skip kalau TEST_DSN tidak di-set (lihat shared/testdb).

func newDB(t *testing.T) *gorm.DB {
	t.Helper()
	return testdb.Open(t, "link", &entity.Link{}, &entity.BlockedDomain{})
}

func mkLink(t *testing.T, repo LinkRepository, userID uuid.UUID, shortCode string, alias *string) *entity.Link {
	t.Helper()
	l := &entity.Link{UserID: userID, OriginalURL: "https://example.com/" + shortCode, ShortCode: shortCode, CustomAlias: alias, IsActive: true}
	if err := repo.Create(context.Background(), l); err != nil {
		t.Fatal(err)
	}
	return l
}

func TestLinkRepository_LookupByCodeAndAlias(t *testing.T) {
	repo := NewLinkRepository(newDB(t))
	ctx := context.Background()
	user := uuid.New()
	alias := "promo"
	l := mkLink(t, repo, user, "abc123", &alias)

	t.Run("by id", func(t *testing.T) {
		got, err := repo.GetByID(ctx, l.ID)
		if err != nil || got == nil || got.ShortCode != "abc123" {
			t.Fatalf("GetByID = %+v, %v", got, err)
		}
		if got, err := repo.GetByID(ctx, uuid.New()); got != nil || err != nil {
			t.Errorf("GetByID(unknown) = %v, %v", got, err)
		}
	})

	t.Run("by id is scoped to the owner", func(t *testing.T) {
		if got, err := repo.GetByIDAndUserID(ctx, l.ID, user); err != nil || got == nil {
			t.Fatalf("owner lookup = %v, %v", got, err)
		}
		if got, err := repo.GetByIDAndUserID(ctx, l.ID, uuid.New()); got != nil || err != nil {
			t.Errorf("another user must not see the link: %v, %v", got, err)
		}
	})

	t.Run("by short code and alias", func(t *testing.T) {
		if got, err := repo.GetByShortCode(ctx, "abc123"); err != nil || got == nil || got.ID != l.ID {
			t.Fatalf("GetByShortCode = %v, %v", got, err)
		}
		if got, err := repo.GetByCustomAlias(ctx, "promo"); err != nil || got == nil || got.ID != l.ID {
			t.Fatalf("GetByCustomAlias = %v, %v", got, err)
		}
		if got, err := repo.GetByShortCode(ctx, "nope"); got != nil || err != nil {
			t.Errorf("GetByShortCode(unknown) = %v, %v", got, err)
		}
		if got, err := repo.GetByCustomAlias(ctx, "nope"); got != nil || err != nil {
			t.Errorf("GetByCustomAlias(unknown) = %v, %v", got, err)
		}
	})

	t.Run("inactive links do not resolve", func(t *testing.T) {
		l.IsActive = false
		if err := repo.Update(ctx, l); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.GetByShortCode(ctx, "abc123"); got != nil {
			t.Error("inactive link must not resolve by short code")
		}
		if got, _ := repo.GetByCustomAlias(ctx, "promo"); got != nil {
			t.Error("inactive link must not resolve by alias")
		}
	})
}

func TestLinkRepository_UniqueConstraints(t *testing.T) {
	repo := NewLinkRepository(newDB(t))
	user := uuid.New()
	alias := "taken"
	mkLink(t, repo, user, "code01", &alias)

	dupCode := &entity.Link{UserID: user, OriginalURL: "https://x.com", ShortCode: "code01", IsActive: true}
	if err := repo.Create(context.Background(), dupCode); err == nil {
		t.Error("duplicate short code must be rejected")
	}
	dupAlias := &entity.Link{UserID: user, OriginalURL: "https://x.com", ShortCode: "code02", CustomAlias: &alias, IsActive: true}
	if err := repo.Create(context.Background(), dupAlias); err == nil {
		t.Error("duplicate custom alias must be rejected")
	}
	// Banyak link tanpa alias tidak boleh bentrok satu sama lain
	mkLink(t, repo, user, "code03", nil)
	mkLink(t, repo, user, "code04", nil)
}

func TestLinkRepository_ListCountAndIDs(t *testing.T) {
	repo := NewLinkRepository(newDB(t))
	ctx := context.Background()
	user, other := uuid.New(), uuid.New()

	var created []*entity.Link
	for _, code := range []string{"l-1", "l-2", "l-3"} {
		created = append(created, mkLink(t, repo, user, code, nil))
		time.Sleep(10 * time.Millisecond) // created_at berbeda supaya urutan DESC pasti
	}
	mkLink(t, repo, other, "l-x", nil)

	n, err := repo.CountByUserID(ctx, user)
	if err != nil || n != 3 {
		t.Fatalf("CountByUserID = %d, %v", n, err)
	}

	page1, err := repo.ListByUserID(ctx, user, 1, 2)
	if err != nil || len(page1) != 2 || page1[0].ShortCode != "l-3" || page1[1].ShortCode != "l-2" {
		t.Fatalf("page 1 = %+v, %v", page1, err)
	}
	page2, err := repo.ListByUserID(ctx, user, 2, 2)
	if err != nil || len(page2) != 1 || page2[0].ShortCode != "l-1" {
		t.Fatalf("page 2 = %+v, %v", page2, err)
	}

	ids, err := repo.GetIDsByUserID(ctx, user)
	if err != nil || len(ids) != 3 {
		t.Fatalf("GetIDsByUserID = %v, %v", ids, err)
	}
	want := map[uuid.UUID]bool{}
	for _, l := range created {
		want[l.ID] = true
	}
	for _, id := range ids {
		if !want[id] {
			t.Errorf("unexpected id %s (belongs to another user?)", id)
		}
	}
}

func TestLinkRepository_SoftDelete(t *testing.T) {
	repo := NewLinkRepository(newDB(t))
	ctx := context.Background()
	user := uuid.New()
	l := mkLink(t, repo, user, "del-me", nil)

	if err := repo.SoftDelete(ctx, l.ID, uuid.New()); err == nil {
		t.Error("another user must not be able to delete the link")
	}
	if err := repo.SoftDelete(ctx, l.ID, user); err != nil {
		t.Fatal(err)
	}
	if got, _ := repo.GetByID(ctx, l.ID); got != nil {
		t.Error("deleted link must disappear from lookups")
	}
	if n, _ := repo.CountByUserID(ctx, user); n != 0 {
		t.Errorf("count after delete = %d, want 0", n)
	}
	if err := repo.SoftDelete(ctx, l.ID, user); err == nil {
		t.Error("deleting twice must report not found")
	}
}

func TestLinkRepository_IncrementClickCountIsAtomic(t *testing.T) {
	repo := NewLinkRepository(newDB(t))
	ctx := context.Background()
	l := mkLink(t, repo, uuid.New(), "hits", nil)

	done := make(chan error, 20)
	for i := 0; i < 20; i++ {
		go func() { done <- repo.IncrementClickCount(ctx, l.ID) }()
	}
	for i := 0; i < 20; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	got, _ := repo.GetByID(ctx, l.ID)
	if got.ClickCount != 20 {
		t.Errorf("click_count = %d, want 20 (lost update?)", got.ClickCount)
	}
}

func TestDomainRepository_IsBlocked(t *testing.T) {
	db := newDB(t)
	repo := NewDomainRepository(db)
	ctx := context.Background()
	if err := db.Create(&entity.BlockedDomain{Domain: "evil.test", Reason: "phishing"}).Error; err != nil {
		t.Fatal(err)
	}

	if blocked, err := repo.IsBlocked(ctx, "evil.test"); err != nil || !blocked {
		t.Errorf("evil.test: blocked=%v err=%v", blocked, err)
	}
	if blocked, err := repo.IsBlocked(ctx, "good.test"); err != nil || blocked {
		t.Errorf("good.test: blocked=%v err=%v", blocked, err)
	}
}
