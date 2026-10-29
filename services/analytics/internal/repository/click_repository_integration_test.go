package repository

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/deaprima/linkforge/services/analytics/internal/entity"
	"github.com/deaprima/linkforge/services/shared/testdb"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Integration test terhadap Postgres sungguhan (SQL di repository memakai fitur Postgres:
// ON CONFLICT, DISTINCT ON, CTE). Di-skip kalau TEST_DSN tidak di-set (lihat shared/testdb);
// setiap test memakai schema sementara sendiri yang di-drop setelah selesai.
func newTestRepo(t *testing.T) (ClickRepository, *gorm.DB) {
	t.Helper()
	db := testdb.Open(t, "analytics", &entity.LinkClick{}, &entity.LinkDailyStat{})
	return NewClickRepository(db), db
}

func newClick(linkID uuid.UUID, at time.Time, ip, country, browser, device, alias string) *entity.LinkClick {
	return &entity.LinkClick{
		EventID: uuid.New(), LinkID: linkID, Alias: alias, IP: ip,
		Country: country, Browser: browser, OS: "Unknown", Device: device, ClickedAt: at,
	}
}

func dailyCount(t *testing.T, db *gorm.DB, linkID uuid.UUID, day string) int64 {
	t.Helper()
	var n int64
	db.Raw("SELECT COALESCE(SUM(click_count),0) FROM link_daily_stats WHERE link_id = ? AND date = ?", linkID, day).Scan(&n)
	return n
}

func TestSaveClick_IsIdempotent(t *testing.T) {
	repo, db := newTestRepo(t)
	ctx := context.Background()
	linkID := uuid.New()
	at := time.Date(2026, 7, 1, 23, 30, 0, 0, time.UTC)
	click := newClick(linkID, at, "1.1.1.1", "ID", "Chrome", "mobile", "promo")

	inserted, err := repo.SaveClick(ctx, click)
	if err != nil || !inserted {
		t.Fatalf("first save: inserted=%v err=%v", inserted, err)
	}

	// Redelivery: event_id sama, objek baru (seperti hasil decode ulang dari Kafka)
	again := *click
	again.ID = uuid.Nil
	inserted, err = repo.SaveClick(ctx, &again)
	if err != nil || inserted {
		t.Fatalf("redelivered save: inserted=%v err=%v, want inserted=false", inserted, err)
	}

	var rows int64
	db.Model(&entity.LinkClick{}).Where("link_id = ?", linkID).Count(&rows)
	if rows != 1 {
		t.Errorf("link_clicks rows = %d, want 1", rows)
	}
	if n := dailyCount(t, db, linkID, "2026-07-01"); n != 1 {
		t.Errorf("daily rollup = %d, want 1 (must not be incremented for duplicates)", n)
	}
}

func TestSaveClick_RollupPerUTCDay(t *testing.T) {
	repo, db := newTestRepo(t)
	ctx := context.Background()
	linkID := uuid.New()
	jakarta := time.FixedZone("WIB", 7*3600)

	for _, at := range []time.Time{
		time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 7, 1, 23, 59, 0, 0, time.UTC),
		time.Date(2026, 7, 2, 5, 0, 0, 0, jakarta), // = 1 Juli 22:00 UTC
		time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC),
	} {
		if _, err := repo.SaveClick(ctx, newClick(linkID, at, "1.1.1.1", "ID", "Chrome", "mobile", "a")); err != nil {
			t.Fatal(err)
		}
	}
	if n := dailyCount(t, db, linkID, "2026-07-01"); n != 3 {
		t.Errorf("2026-07-01 = %d, want 3", n)
	}
	if n := dailyCount(t, db, linkID, "2026-07-02"); n != 1 {
		t.Errorf("2026-07-02 = %d, want 1", n)
	}
}

func TestLinkQueries(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()
	linkID, other := uuid.New(), uuid.New()
	d := func(day, hour int) time.Time { return time.Date(2026, 7, day, hour, 0, 0, 0, time.UTC) }

	seed := []*entity.LinkClick{
		newClick(linkID, d(1, 9), "1.1.1.1", "ID", "Chrome", "mobile", "a"),
		newClick(linkID, d(1, 10), "1.1.1.1", "ID", "Chrome", "mobile", "a"), // IP sama → unique tetap 1
		newClick(linkID, d(2, 9), "2.2.2.2", "ID", "Safari", "mobile", "a"),
		newClick(linkID, d(3, 9), "3.3.3.3", "US", "Chrome", "desktop", "a"),
		newClick(linkID, d(5, 9), "4.4.4.4", "SG", "Firefox", "desktop", "a"), // di luar rentang
		newClick(other, d(2, 9), "5.5.5.5", "JP", "Chrome", "mobile", "b"),    // link lain
	}
	for _, c := range seed {
		if _, err := repo.SaveClick(ctx, c); err != nil {
			t.Fatal(err)
		}
	}
	from, to := d(1, 0), d(4, 0) // [1 Juli, 4 Juli)

	totals, err := repo.Totals(ctx, linkID, from, to)
	if err != nil || totals.Total != 4 || totals.Unique != 3 {
		t.Errorf("totals = %+v, err %v; want total=4 unique=3", totals, err)
	}

	countries, err := repo.TopBy(ctx, linkID, ByCountry, from, to, 5)
	if err != nil || fmt.Sprint(countries) != "[{ID 3} {US 1}]" {
		t.Errorf("countries = %v, err %v", countries, err)
	}
	browsers, _ := repo.TopBy(ctx, linkID, ByBrowser, from, to, 1)
	if fmt.Sprint(browsers) != "[{Chrome 3}]" {
		t.Errorf("browsers (limit 1) = %v", browsers)
	}
	devices, _ := repo.TopBy(ctx, linkID, ByDevice, from, to, 5)
	if fmt.Sprint(devices) != "[{mobile 3} {desktop 1}]" {
		t.Errorf("devices = %v", devices)
	}
	if _, err := repo.TopBy(ctx, linkID, Breakdown("ip; DROP TABLE link_clicks"), from, to, 5); err == nil {
		t.Error("non-whitelisted breakdown must be rejected")
	}

	days, err := repo.DailyClicks(ctx, linkID, from, to)
	if err != nil || len(days) != 3 {
		t.Fatalf("days = %v, err %v", days, err)
	}
	if !days[0].Date.Equal(d(1, 0)) || days[0].Clicks != 2 || days[2].Clicks != 1 {
		t.Errorf("days = %+v", days)
	}
}

func TestSummaryQueries(t *testing.T) {
	repo, _ := newTestRepo(t)
	ctx := context.Background()
	popular, quiet, unclicked := uuid.New(), uuid.New(), uuid.New()
	at := time.Date(2026, 7, 1, 9, 0, 0, 0, time.UTC)

	for i := range 3 {
		alias := "old-alias"
		if i == 2 {
			alias = "new-alias" // alias diganti via PUT, klik terakhir memakai alias baru
		}
		repo.SaveClick(ctx, newClick(popular, at.Add(time.Duration(i)*time.Hour), "1.1.1.1", "ID", "Chrome", "mobile", alias))
	}
	repo.SaveClick(ctx, newClick(quiet, at, "1.1.1.1", "ID", "Chrome", "mobile", "quiet"))

	ids := []uuid.UUID{popular, quiet, unclicked}
	total, err := repo.TotalClicks(ctx, ids)
	if err != nil || total != 4 {
		t.Errorf("total = %d, err %v; want 4", total, err)
	}

	top, err := repo.TopLinks(ctx, ids, 10)
	if err != nil || len(top) != 2 {
		t.Fatalf("top = %v, err %v", top, err)
	}
	if top[0].LinkID != popular || top[0].Clicks != 3 || top[0].Alias != "new-alias" {
		t.Errorf("top[0] = %+v, want popular link with latest alias", top[0])
	}
	if top[1].LinkID != quiet || top[1].Clicks != 1 {
		t.Errorf("top[1] = %+v", top[1])
	}

	if limited, _ := repo.TopLinks(ctx, ids, 1); len(limited) != 1 {
		t.Errorf("limit not applied: %v", limited)
	}
	if total, err := repo.TotalClicks(ctx, nil); err != nil || total != 0 {
		t.Errorf("empty ids total = %d, %v", total, err)
	}
	if top, err := repo.TopLinks(ctx, nil, 10); err != nil || top != nil {
		t.Errorf("empty ids top = %v, %v", top, err)
	}
}
