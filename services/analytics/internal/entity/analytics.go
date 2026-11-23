package entity

import (
	"time"

	"github.com/google/uuid"
)

// LinkClick adalah raw log satu klik (lihat DATABASE-SCHEMA.md, schema analytics).
type LinkClick struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	EventID   uuid.UUID `gorm:"type:uuid;uniqueIndex;not null"` // kunci idempotency consumer
	LinkID    uuid.UUID `gorm:"type:uuid;index;not null"`       // BUKAN FK ke link.links
	Alias     string    `gorm:"type:varchar(64)"`
	IP        string    `gorm:"type:varchar(64)"`
	Country   string    `gorm:"type:varchar(4);index"`
	City      string    `gorm:"type:varchar(255)"`
	Browser   string    `gorm:"type:varchar(64);index"`
	OS        string    `gorm:"type:varchar(64);index"`
	Device    string    `gorm:"type:varchar(16);index"` // desktop | mobile | tablet | bot
	Referrer  string    `gorm:"type:text"`
	ClickedAt time.Time `gorm:"not null;index"`
}

func (LinkClick) TableName() string {
	return "link_clicks"
}

// LinkDailyStat adalah rollup harian (tanggal UTC) untuk time series dashboard.
type LinkDailyStat struct {
	LinkID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	Date       time.Time `gorm:"type:date;primaryKey"`
	ClickCount int64     `gorm:"not null;default:0"`
}

func (LinkDailyStat) TableName() string {
	return "link_daily_stats"
}
