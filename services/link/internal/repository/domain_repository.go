package repository

import (
    "context"

    "github.com/deaprima/linkforge/services/link/internal/entity"
    "gorm.io/gorm"
)

type DomainRepository interface {
    IsBlocked(ctx context.Context, domain string) (bool, error)
}

type domainRepository struct {
    db *gorm.DB
}

func NewDomainRepository(db *gorm.DB) DomainRepository {
    return &domainRepository{db: db}
}

func (r *domainRepository) IsBlocked(ctx context.Context, domain string) (bool, error) {
    var count int64
    err := r.db.WithContext(ctx).Model(&entity.BlockedDomain{}).
        Where("domain = ?", domain).
        Count(&count).Error
    if err != nil {
        return false, err
    }
    return count > 0, nil
}
