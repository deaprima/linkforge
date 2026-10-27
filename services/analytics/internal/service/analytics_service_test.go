package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/deaprima/linkforge/services/analytics/internal/entity"
	"github.com/deaprima/linkforge/services/analytics/internal/repository"
	"github.com/google/uuid"
)

var errDB = errors.New("db down")

// fakeRepo mencatat argumen query dan mengembalikan data yang dikonfigurasi.
type fakeRepo struct {
	totals  repository.LinkTotals
	top     map[repository.Breakdown][]repository.CountItem
	days    []repository.DailyCount
	total   int64
	links   []repository.LinkClicks
	failOn  string
	gotFrom time.Time
	gotTo   time.Time
	gotIDs  []uuid.UUID
}

func (f *fakeRepo) fail(op string) error {
	if f.failOn == op {
		return errDB
	}
	return nil
}

func (f *fakeRepo) SaveClick(context.Context, *entity.LinkClick) (bool, error) { return true, nil }

func (f *fakeRepo) Totals(_ context.Context, _ uuid.UUID, from, to time.Time) (repository.LinkTotals, error) {
	f.gotFrom, f.gotTo = from, to
	return f.totals, f.fail("totals")
}
func (f *fakeRepo) TopBy(_ context.Context, _ uuid.UUID, by repository.Breakdown, _, _ time.Time, _ int) ([]repository.CountItem, error) {
	return f.top[by], f.fail(string(by))
}
func (f *fakeRepo) DailyClicks(context.Context, uuid.UUID, time.Time, time.Time) ([]repository.DailyCount, error) {
	return f.days, f.fail("daily")
}
func (f *fakeRepo) TotalClicks(_ context.Context, ids []uuid.UUID) (int64, error) {
	f.gotIDs = ids
	return f.total, f.fail("total")
}
func (f *fakeRepo) TopLinks(context.Context, []uuid.UUID, int) ([]repository.LinkClicks, error) {
	return f.links, f.fail("toplinks")
}

func date(s string) time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return t
}

func newTestService(repo *fakeRepo) *analyticsService {
	return &analyticsService{repo: repo, now: func() time.Time { return time.Date(2026, 10, 27, 15, 30, 0, 0, time.UTC) }}
}

func TestGetLinkAnalytics(t *testing.T) {
	repo := &fakeRepo{
		totals: repository.LinkTotals{Total: 254, Unique: 180},
		top: map[repository.Breakdown][]repository.CountItem{
			repository.ByCountry: {{Key: "ID", Count: 120}},
			repository.ByBrowser: {{Key: "Chrome", Count: 150}},
			repository.ByDevice:  {{Key: "mobile", Count: 180}},
		},
		days: []repository.DailyCount{{Date: date("2026-07-02"), Clicks: 30}},
	}
	svc := newTestService(repo)

	res, err := svc.GetLinkAnalytics(context.Background(), uuid.NewString(), "2026-07-01", "2026-07-03")
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalClicks != 254 || res.UniqueClicks != 180 || res.TopCountries[0].Key != "ID" ||
		res.TopBrowsers[0].Key != "Chrome" || res.TopDevices[0].Key != "mobile" {
		t.Errorf("result = %+v", res)
	}

	// Rentang [from, to+1 hari) — hari terakhir ikut terhitung penuh
	if !repo.gotFrom.Equal(date("2026-07-01")) || !repo.gotTo.Equal(date("2026-07-04")) {
		t.Errorf("query range = [%v, %v)", repo.gotFrom, repo.gotTo)
	}

	// Hari tanpa klik diisi 0
	want := []int64{0, 30, 0}
	if len(res.TimeSeries) != 3 {
		t.Fatalf("time series has %d days, want 3", len(res.TimeSeries))
	}
	for i, d := range res.TimeSeries {
		if d.Clicks != want[i] || !d.Date.Equal(date("2026-07-01").AddDate(0, 0, i)) {
			t.Errorf("day %d = %+v", i, d)
		}
	}
}

func TestGetLinkAnalytics_DefaultRangeIsLast30Days(t *testing.T) {
	repo := &fakeRepo{}
	res, err := newTestService(repo).GetLinkAnalytics(context.Background(), uuid.NewString(), "", "")
	if err != nil {
		t.Fatal(err)
	}
	if !repo.gotFrom.Equal(date("2026-09-28")) || !repo.gotTo.Equal(date("2026-10-28")) {
		t.Errorf("default range = [%v, %v)", repo.gotFrom, repo.gotTo)
	}
	if len(res.TimeSeries) != 30 {
		t.Errorf("time series has %d days, want 30", len(res.TimeSeries))
	}
}

func TestGetLinkAnalytics_InvalidInput(t *testing.T) {
	tests := []struct {
		name, linkID, from, to, wantMsg string
	}{
		{"bad link id", "nope", "", "", "invalid link_id"},
		{"bad from", uuid.NewString(), "07/01/2026", "", "from must be a date"},
		{"bad to", uuid.NewString(), "", "2026-13-01", "to must be a date"},
		{"from after to", uuid.NewString(), "2026-07-10", "2026-07-01", "must not be after"},
		{"range too long", uuid.NewString(), "2025-01-01", "2026-07-01", "must not exceed 366 days"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newTestService(&fakeRepo{}).GetLinkAnalytics(context.Background(), tt.linkID, tt.from, tt.to)
			if !errors.Is(err, ErrInvalidArgument) || !strings.Contains(err.Error(), tt.wantMsg) {
				t.Errorf("err = %v, want InvalidArgument containing %q", err, tt.wantMsg)
			}
		})
	}

	// Batas maksimal tepat 366 hari masih boleh
	if _, err := newTestService(&fakeRepo{}).GetLinkAnalytics(context.Background(), uuid.NewString(), "2025-07-01", "2026-07-01"); err != nil {
		t.Errorf("366-day range must be allowed: %v", err)
	}
}

func TestGetLinkAnalytics_RepositoryErrors(t *testing.T) {
	for _, op := range []string{"totals", "country", "browser", "device", "daily"} {
		t.Run(op, func(t *testing.T) {
			_, err := newTestService(&fakeRepo{failOn: op}).GetLinkAnalytics(context.Background(), uuid.NewString(), "", "")
			if !errors.Is(err, errDB) || errors.Is(err, ErrInvalidArgument) {
				t.Errorf("err = %v, want wrapped db error", err)
			}
		})
	}
}

func TestGetSummary(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	repo := &fakeRepo{total: 1280, links: []repository.LinkClicks{{LinkID: a, Alias: "my-alias", Clicks: 254}}}

	res, err := newTestService(repo).GetSummary(context.Background(), []string{a.String(), b.String()})
	if err != nil {
		t.Fatal(err)
	}
	if res.TotalLinks != 2 || res.TotalClicks != 1280 || res.TopLinks[0].Alias != "my-alias" {
		t.Errorf("summary = %+v", res)
	}
	if len(repo.gotIDs) != 2 || repo.gotIDs[0] != a {
		t.Errorf("repo got ids %v", repo.gotIDs)
	}

	empty, err := newTestService(&fakeRepo{}).GetSummary(context.Background(), nil)
	if err != nil || empty.TotalLinks != 0 || empty.TotalClicks != 0 {
		t.Errorf("empty summary = %+v, %v", empty, err)
	}

	_, err = newTestService(&fakeRepo{}).GetSummary(context.Background(), []string{"bad"})
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("invalid id err = %v", err)
	}
	_, err = newTestService(&fakeRepo{}).GetSummary(context.Background(), make([]string, maxSummaryLink+1))
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("too many ids err = %v", err)
	}
	for _, op := range []string{"total", "toplinks"} {
		if _, err := newTestService(&fakeRepo{failOn: op}).GetSummary(context.Background(), []string{a.String()}); !errors.Is(err, errDB) {
			t.Errorf("%s failure err = %v", op, err)
		}
	}
}

func TestNewAnalyticsService(t *testing.T) {
	if NewAnalyticsService(&fakeRepo{}) == nil {
		t.Fatal("nil service")
	}
}
