package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Link struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID       uuid.UUID      `gorm:"type:uuid;index;not null"`
	OriginalURL  string         `gorm:"type:text;not null"`
	ShortCode    string         `gorm:"type:varchar(16);uniqueIndex;not null"`
	CustomAlias  *string        `gorm:"type:varchar(64);uniqueIndex"`
	Title        string         `gorm:"type:varchar(255)"`
	PasswordHash *string        `gorm:"type:varchar(255)"`
	ExpiredAt    *time.Time     `gorm:"index"`
	IsActive     bool           `gorm:"not null;default:true"`
	ClickCount   int64          `gorm:"not null;default:0"`
	CreatedAt    time.Time      `gorm:"not null;default:now()"`
	UpdatedAt    time.Time      `gorm:"not null;default:now()"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (Link) TableName() string {
	return "links"
}

type BlockedDomain struct {
	Domain    string    `gorm:"type:varchar(255);primaryKey"`
	Reason    string    `gorm:"type:varchar(255)"`
	CreatedAt time.Time `gorm:"not null;default:now()"`
}

func (BlockedDomain) TableName() string {
	return "blocked_domains"
}
