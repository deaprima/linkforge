package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/deaprima/linkforge/services/analytics/internal/repository"
	"github.com/google/uuid"
)

const (
	dateLayout     = "2006-01-02"
	defaultRange   = 30  // hari, kalau from/to tidak dikirim
	maxRangeDays   = 366 // batas query supaya GROUP BY di link_clicks tetap ringan
	topBreakdownN  = 5
	topLinksN      = 10
	maxSummaryLink = 10000
)

// ErrInvalidArgument menandai error input (dipetakan ke codes.InvalidArgument).
var ErrInvalidArgument = errors.New("invalid argument")

type LinkAnalytics struct {
	LinkID       uuid.UUID
	TotalClicks  int64
	UniqueClicks int64
	TopCountries []repository.CountItem
	TopBrowsers  []repository.CountItem
	TopDevices   []repository.CountItem
	TimeSeries   []repository.DailyCount // satu entri per hari, termasuk hari tanpa klik
}

type Summary struct {
	TotalLinks  int
	TotalClicks int64
	TopLinks    []repository.LinkClicks
}

type AnalyticsService interface {
	GetLinkAnalytics(ctx context.Context, linkID, from, to string) (*LinkAnalytics, error)
	GetSummary(ctx context.Context, linkIDs []string) (*Summary, error)
}

type analyticsService struct {
	repo repository.ClickRepository
	now  func() time.Time
}

func NewAnalyticsService(repo repository.ClickRepository) AnalyticsService {
	return &analyticsService{repo: repo, now: time.Now}
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidArgument, fmt.Sprintf(format, args...))
}

// parseRange mengembalikan [from, to) dalam UTC; to = awal hari setelah tanggal akhir.
func (s *analyticsService) parseRange(fromStr, toStr string) (time.Time, time.Time, error) {
	today := s.now().UTC().Truncate(24 * time.Hour)

	to := today
	if toStr != "" {
		t, err := time.Parse(dateLayout, toStr)
		if err != nil {
			return time.Time{}, time.Time{}, invalid("to must be a date in YYYY-MM-DD format")
		}
		to = t
	}
	from := to.AddDate(0, 0, -(defaultRange - 1))
	if fromStr != "" {
		f, err := time.Parse(dateLayout, fromStr)
		if err != nil {
			return time.Time{}, time.Time{}, invalid("from must be a date in YYYY-MM-DD format")
		}
		from = f
	}

	if from.After(to) {
		return time.Time{}, time.Time{}, invalid("from must not be after to")
	}
	if days := int(to.Sub(from).Hours()/24) + 1; days > maxRangeDays {
		return time.Time{}, time.Time{}, invalid("date range must not exceed %d days", maxRangeDays)
	}
	return from, to.AddDate(0, 0, 1), nil
}

func (s *analyticsService) GetLinkAnalytics(ctx context.Context, linkIDStr, fromStr, toStr string) (*LinkAnalytics, error) {
	linkID, err := uuid.Parse(linkIDStr)
	if err != nil {
		return nil, invalid("invalid link_id format")
	}
	from, to, err := s.parseRange(fromStr, toStr)
	if err != nil {
		return nil, err
	}

	totals, err := s.repo.Totals(ctx, linkID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to count clicks: %w", err)
	}
	result := &LinkAnalytics{LinkID: linkID, TotalClicks: totals.Total, UniqueClicks: totals.Unique}

	breakdowns := []struct {
		by  repository.Breakdown
		dst *[]repository.CountItem
	}{
		{repository.ByCountry, &result.TopCountries},
		{repository.ByBrowser, &result.TopBrowsers},
		{repository.ByDevice, &result.TopDevices},
	}
	for _, b := range breakdowns {
		items, err := s.repo.TopBy(ctx, linkID, b.by, from, to, topBreakdownN)
		if err != nil {
			return nil, fmt.Errorf("failed to aggregate by %s: %w", b.by, err)
		}
		*b.dst = items
	}

	days, err := s.repo.DailyClicks(ctx, linkID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to load time series: %w", err)
	}
	result.TimeSeries = fillDays(days, from, to)
	return result, nil
}

// fillDays melengkapi hari tanpa klik dengan 0 supaya chart di dashboard tidak bolong.
func fillDays(days []repository.DailyCount, from, to time.Time) []repository.DailyCount {
	byDate := make(map[string]int64, len(days))
	for _, d := range days {
		byDate[d.Date.UTC().Format(dateLayout)] = d.Clicks
	}
	var out []repository.DailyCount
	for d := from; d.Before(to); d = d.AddDate(0, 0, 1) {
		out = append(out, repository.DailyCount{Date: d, Clicks: byDate[d.Format(dateLayout)]})
	}
	return out
}

// GetSummary: link_ids dikirim Gateway (API composition dari Link Service), karena
// Analytics tidak tahu link mana milik user siapa.
func (s *analyticsService) GetSummary(ctx context.Context, linkIDStrs []string) (*Summary, error) {
	if len(linkIDStrs) > maxSummaryLink {
		return nil, invalid("too many link_ids (max %d)", maxSummaryLink)
	}
	linkIDs := make([]uuid.UUID, 0, len(linkIDStrs))
	for _, raw := range linkIDStrs {
		id, err := uuid.Parse(raw)
		if err != nil {
			return nil, invalid("invalid link_id %q", raw)
		}
		linkIDs = append(linkIDs, id)
	}

	total, err := s.repo.TotalClicks(ctx, linkIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to count clicks: %w", err)
	}
	top, err := s.repo.TopLinks(ctx, linkIDs, topLinksN)
	if err != nil {
		return nil, fmt.Errorf("failed to load top links: %w", err)
	}
	return &Summary{TotalLinks: len(linkIDs), TotalClicks: total, TopLinks: top}, nil
}
