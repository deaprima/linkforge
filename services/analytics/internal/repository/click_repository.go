package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/deaprima/linkforge/services/analytics/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Hasil query agregasi

type CountItem struct {
	Key   string
	Count int64
}

type DailyCount struct {
	Date   time.Time
	Clicks int64
}

type LinkTotals struct {
	Total  int64
	Unique int64
}

type LinkClicks struct {
	LinkID uuid.UUID
	Alias  string
	Clicks int64
}

// Breakdown adalah kolom yang boleh dipakai untuk GROUP BY (whitelist — nama kolom tidak bisa di-bind sebagai parameter).
type Breakdown string

const (
	ByCountry Breakdown = "country"
	ByBrowser Breakdown = "browser"
	ByDevice  Breakdown = "device"
)

type ClickRepository interface {
	// SaveClick menyimpan klik secara idempotent. inserted=false berarti event_id sudah pernah diproses.
	SaveClick(ctx context.Context, click *entity.LinkClick) (inserted bool, err error)

	Totals(ctx context.Context, linkID uuid.UUID, from, to time.Time) (LinkTotals, error)
	TopBy(ctx context.Context, linkID uuid.UUID, by Breakdown, from, to time.Time, limit int) ([]CountItem, error)
	DailyClicks(ctx context.Context, linkID uuid.UUID, from, to time.Time) ([]DailyCount, error)

	TotalClicks(ctx context.Context, linkIDs []uuid.UUID) (int64, error)
	TopLinks(ctx context.Context, linkIDs []uuid.UUID, limit int) ([]LinkClicks, error)
}

type clickRepository struct {
	db *gorm.DB
}

func NewClickRepository(db *gorm.DB) ClickRepository {
	return &clickRepository{db: db}
}

func (r *clickRepository) SaveClick(ctx context.Context, click *entity.LinkClick) (bool, error) {
	inserted := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "event_id"}},
			DoNothing: true,
		}).Create(click)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return nil // duplikat (redelivery Kafka) — rollup tidak boleh di-increment lagi
		}
		inserted = true

		return tx.Exec(`
			INSERT INTO link_daily_stats (link_id, date, click_count) VALUES (?, ?, 1)
			ON CONFLICT (link_id, date) DO UPDATE SET click_count = link_daily_stats.click_count + 1`,
			click.LinkID, click.ClickedAt.UTC().Format("2006-01-02"),
		).Error
	})
	if err != nil {
		return false, fmt.Errorf("save click: %w", err)
	}
	return inserted, nil
}

// from inklusif, to eksklusif (lihat service: to = awal hari setelah tanggal akhir)

func (r *clickRepository) Totals(ctx context.Context, linkID uuid.UUID, from, to time.Time) (LinkTotals, error) {
	var t LinkTotals
	err := r.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) AS total, COUNT(DISTINCT ip) AS "unique"
		FROM link_clicks
		WHERE link_id = ? AND clicked_at >= ? AND clicked_at < ?`,
		linkID, from, to,
	).Scan(&t).Error
	return t, err
}

func (r *clickRepository) TopBy(ctx context.Context, linkID uuid.UUID, by Breakdown, from, to time.Time, limit int) ([]CountItem, error) {
	switch by {
	case ByCountry, ByBrowser, ByDevice:
	default:
		return nil, fmt.Errorf("unsupported breakdown %q", by)
	}
	var items []CountItem
	err := r.db.WithContext(ctx).Raw(fmt.Sprintf(`
		SELECT %[1]s AS key, COUNT(*) AS count
		FROM link_clicks
		WHERE link_id = ? AND clicked_at >= ? AND clicked_at < ?
		GROUP BY %[1]s
		ORDER BY count DESC, key
		LIMIT ?`, by),
		linkID, from, to, limit,
	).Scan(&items).Error
	return items, err
}

func (r *clickRepository) DailyClicks(ctx context.Context, linkID uuid.UUID, from, to time.Time) ([]DailyCount, error) {
	var days []DailyCount
	err := r.db.WithContext(ctx).Raw(`
		SELECT date, click_count AS clicks
		FROM link_daily_stats
		WHERE link_id = ? AND date >= ? AND date < ?
		ORDER BY date`,
		linkID, from.Format("2006-01-02"), to.Format("2006-01-02"),
	).Scan(&days).Error
	return days, err
}

func (r *clickRepository) TotalClicks(ctx context.Context, linkIDs []uuid.UUID) (int64, error) {
	if len(linkIDs) == 0 {
		return 0, nil
	}
	var total int64
	err := r.db.WithContext(ctx).Raw(
		`SELECT COALESCE(SUM(click_count), 0) FROM link_daily_stats WHERE link_id IN ?`, linkIDs,
	).Scan(&total).Error
	return total, err
}

func (r *clickRepository) TopLinks(ctx context.Context, linkIDs []uuid.UUID, limit int) ([]LinkClicks, error) {
	if len(linkIDs) == 0 {
		return nil, nil
	}
	// Alias diambil dari klik terakhir — alias bisa berubah lewat PUT /links/:id
	var links []LinkClicks
	err := r.db.WithContext(ctx).Raw(`
		WITH totals AS (
			SELECT link_id, SUM(click_count) AS clicks
			FROM link_daily_stats
			WHERE link_id IN ?
			GROUP BY link_id
		),
		latest_alias AS (
			SELECT DISTINCT ON (link_id) link_id, alias
			FROM link_clicks
			WHERE link_id IN ?
			ORDER BY link_id, clicked_at DESC
		)
		SELECT t.link_id, COALESCE(a.alias, '') AS alias, t.clicks
		FROM totals t
		LEFT JOIN latest_alias a ON a.link_id = t.link_id
		ORDER BY t.clicks DESC, t.link_id
		LIMIT ?`,
		linkIDs, linkIDs, limit,
	).Scan(&links).Error
	return links, err
}
