package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Email        string         `gorm:"type:varchar(255);uniqueIndex;not null"`
	PasswordHash *string        `gorm:"type:varchar(255)"`
	Name         string         `gorm:"type:varchar(255);not null"`
	CreatedAt    time.Time      `gorm:"not null;default:now()"`
	UpdatedAt    time.Time      `gorm:"not null;default:now()"`
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (User) TableName() string {
	return "users"
}

type OAuthAccount struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID         uuid.UUID `gorm:"type:uuid;not null"`
	User           User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Provider       string    `gorm:"type:varchar(32);not null;uniqueIndex:idx_provider_user"`
	ProviderUserID string    `gorm:"type:varchar(255);not null;uniqueIndex:idx_provider_user"`
	Email          string    `gorm:"type:varchar(255);not null"`
	CreatedAt      time.Time `gorm:"not null;default:now()"`
}

func (OAuthAccount) TableName() string {
	return "oauth_accounts"
}

type RefreshToken struct {
	ID        uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID    uuid.UUID  `gorm:"type:uuid;index;not null"`
	User      User       `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	TokenHash string     `gorm:"type:varchar(255);uniqueIndex;not null"`
	ExpiresAt time.Time  `gorm:"not null"`
	RevokedAt *time.Time `gorm:"index"`
	UserAgent string     `gorm:"type:text"`
	IPAddress string     `gorm:"type:varchar(64)"`
	CreatedAt time.Time  `gorm:"not null;default:now()"`
}

func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

type ApiKey struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID     uuid.UUID  `gorm:"type:uuid;index;not null"`
	User       User       `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
	Name       string     `gorm:"type:varchar(255);not null"`
	KeyHash    string     `gorm:"type:varchar(255);uniqueIndex;not null"`
	KeyPrefix  string     `gorm:"type:varchar(16);not null"`
	LastUsedAt *time.Time `gorm:"index"`
	RevokedAt  *time.Time `gorm:"index"`
	CreatedAt  time.Time  `gorm:"not null;default:now()"`
}

func (ApiKey) TableName() string {
	return "api_keys"
}
