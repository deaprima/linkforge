package repository

import (
	"context"
	"errors"
	"time"

	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ApiKeyRepository interface {
	Create(ctx context.Context, key *entity.ApiKey) error
	GetByID(ctx context.Context, id uuid.UUID) (*entity.ApiKey, error)
	GetByHash(ctx context.Context, keyHash string) (*entity.ApiKey, error)
	ListByUserID(ctx context.Context, userID uuid.UUID) ([]entity.ApiKey, error)
	Revoke(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	UpdateLastUsed(ctx context.Context, id uuid.UUID) error
}

type apiKeyRepository struct {
	db *gorm.DB
}

func NewApiKeyRepository(db *gorm.DB) ApiKeyRepository {
	return &apiKeyRepository{db: db}
}

func (r *apiKeyRepository) Create(ctx context.Context, key *entity.ApiKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

func (r *apiKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.ApiKey, error) {
	var key entity.ApiKey
	err := r.db.WithContext(ctx).First(&key, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &key, nil
}

func (r *apiKeyRepository) GetByHash(ctx context.Context, keyHash string) (*entity.ApiKey, error) {
	var key entity.ApiKey
	// Hanya ambil key yang belum di-revoke
	err := r.db.WithContext(ctx).First(&key, "key_hash = ? AND revoked_at IS NULL", keyHash).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &key, nil
}

func (r *apiKeyRepository) ListByUserID(ctx context.Context, userID uuid.UUID) ([]entity.ApiKey, error) {
	var keys []entity.ApiKey
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Order("created_at DESC").
		Find(&keys).Error
	return keys, err
}

func (r *apiKeyRepository) Revoke(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&entity.ApiKey{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("revoked_at", time.Now()).Error
}

func (r *apiKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&entity.ApiKey{}).
		Where("id = ?", id).
		Update("last_used_at", time.Now()).Error
}
