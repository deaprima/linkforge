package repository

import (
    "context"
    "errors"

    "github.com/deaprima/linkforge/services/auth/internal/entity"
    "gorm.io/gorm"
)

type OAuthRepository interface {
    GetByProviderAndUserID(ctx context.Context, provider, providerUserID string) (*entity.OAuthAccount, error)
    Create(ctx context.Context, account *entity.OAuthAccount) error
}

type oauthRepository struct {
    db *gorm.DB
}

func NewOAuthRepository(db *gorm.DB) OAuthRepository {
    return &oauthRepository{db: db}
}

func (r *oauthRepository) GetByProviderAndUserID(ctx context.Context, provider, providerUserID string) (*entity.OAuthAccount, error) {
    var account entity.OAuthAccount
    err := r.db.WithContext(ctx).
        Where("provider = ? AND provider_user_id = ?", provider, providerUserID).
        First(&account).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, err
    }
    return &account, nil
}

func (r *oauthRepository) Create(ctx context.Context, account *entity.OAuthAccount) error {
    return r.db.WithContext(ctx).Create(account).Error
}
