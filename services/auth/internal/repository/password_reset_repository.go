package repository

import (
	"context"
	"errors"
	"time"

	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PasswordResetRepository interface {
	Create(ctx context.Context, token *entity.PasswordResetToken) error
	GetByHash(ctx context.Context, tokenHash string) (*entity.PasswordResetToken, error)
	// Consume menandai token terpakai; false kalau token sudah terpakai duluan (request paralel).
	Consume(ctx context.Context, id uuid.UUID) (bool, error)
	// InvalidateAllByUserID mematikan semua token milik user yang belum terpakai.
	InvalidateAllByUserID(ctx context.Context, userID uuid.UUID) error
	// CountSince menghitung token yang dibuat untuk user sejak waktu tertentu (batas email per jam).
	CountSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error)
}

type passwordResetRepository struct {
	db *gorm.DB
}

func NewPasswordResetRepository(db *gorm.DB) PasswordResetRepository {
	return &passwordResetRepository{db: db}
}

func (r *passwordResetRepository) Create(ctx context.Context, token *entity.PasswordResetToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

func (r *passwordResetRepository) GetByHash(ctx context.Context, tokenHash string) (*entity.PasswordResetToken, error) {
	var token entity.PasswordResetToken
	err := r.db.WithContext(ctx).First(&token, "token_hash = ?", tokenHash).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &token, nil
}

func (r *passwordResetRepository) Consume(ctx context.Context, id uuid.UUID) (bool, error) {
	res := r.db.WithContext(ctx).Model(&entity.PasswordResetToken{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", time.Now())
	return res.RowsAffected == 1, res.Error
}

func (r *passwordResetRepository) InvalidateAllByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&entity.PasswordResetToken{}).
		Where("user_id = ? AND used_at IS NULL", userID).
		Update("used_at", time.Now()).Error
}

func (r *passwordResetRepository) CountSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&entity.PasswordResetToken{}).
		Where("user_id = ? AND created_at >= ?", userID, since).
		Count(&n).Error
	return n, err
}
