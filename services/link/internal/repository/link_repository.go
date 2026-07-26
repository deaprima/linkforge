package repository

import (
	"context"
	"errors"

	"github.com/deaprima/linkforge/services/link/internal/entity"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type LinkRepository interface {
	Create(ctx context.Context, link *entity.Link) error
    GetByID(ctx context.Context, id uuid.UUID) (*entity.Link, error)
    GetByIDAndUserID(ctx context.Context, id, userID uuid.UUID) (*entity.Link, error)
    ListByUserID(ctx context.Context, userID uuid.UUID, page, limit int) ([]entity.Link, error)
    CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error)
    Update(ctx context.Context, link *entity.Link) error
    SoftDelete(ctx context.Context, id, userID uuid.UUID) error
    GetByShortCode(ctx context.Context, shortCode string) (*entity.Link, error)
    GetByCustomAlias(ctx context.Context, alias string) (*entity.Link, error)
    IncrementClickCount(ctx context.Context, id uuid.UUID) error
    GetIDsByUserID(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}

type linkRepository struct {
	db *gorm.DB
}

func NewLinkRepository(db *gorm.DB) LinkRepository {
    return &linkRepository{db: db}
}

func (r *linkRepository) Create(ctx context.Context, link *entity.Link) error {
	return r.db.WithContext(ctx).Create(link).Error
}

func (r *linkRepository) GetByID(ctx context.Context, id uuid.UUID) (*entity.Link, error) {
    var link entity.Link
    err := r.db.WithContext(ctx).First(&link, "id = ?", id).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, err
    }
    return &link, nil
}
func (r *linkRepository) GetByIDAndUserID(ctx context.Context, id, userID uuid.UUID) (*entity.Link, error) {
    var link entity.Link
    err := r.db.WithContext(ctx).
        First(&link, "id = ? AND user_id = ?", id, userID).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, err
    }
    return &link, nil
}
func (r *linkRepository) ListByUserID(ctx context.Context, userID uuid.UUID, page, limit int) ([]entity.Link, error) {
    var links []entity.Link
    offset := (page - 1) * limit
    err := r.db.WithContext(ctx).
        Where("user_id = ?", userID).
        Order("created_at DESC").
        Offset(offset).Limit(limit).
        Find(&links).Error
    return links, err
}
func (r *linkRepository) CountByUserID(ctx context.Context, userID uuid.UUID) (int64, error) {
    var count int64
    err := r.db.WithContext(ctx).Model(&entity.Link{}).
        Where("user_id = ?", userID).
        Count(&count).Error
    return count, err
}
func (r *linkRepository) Update(ctx context.Context, link *entity.Link) error {
    return r.db.WithContext(ctx).Save(link).Error
}
func (r *linkRepository) SoftDelete(ctx context.Context, id, userID uuid.UUID) error {
    result := r.db.WithContext(ctx).
        Where("id = ? AND user_id = ?", id, userID).
        Delete(&entity.Link{})
    if result.Error != nil {
        return result.Error
    }
    if result.RowsAffected == 0 {
        return errors.New("link not found or not owned by user")
    }
    return nil
}
func (r *linkRepository) GetByShortCode(ctx context.Context, shortCode string) (*entity.Link, error) {
    var link entity.Link
    err := r.db.WithContext(ctx).
        First(&link, "short_code = ? AND is_active = true", shortCode).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, err
    }
    return &link, nil
}
func (r *linkRepository) GetByCustomAlias(ctx context.Context, alias string) (*entity.Link, error) {
    var link entity.Link
    err := r.db.WithContext(ctx).
        First(&link, "custom_alias = ? AND is_active = true", alias).Error
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            return nil, nil
        }
        return nil, err
    }
    return &link, nil
}
func (r *linkRepository) IncrementClickCount(ctx context.Context, id uuid.UUID) error {
    return r.db.WithContext(ctx).Model(&entity.Link{}).
        Where("id = ?", id).
        UpdateColumn("click_count", gorm.Expr("click_count + 1")).Error
}
func (r *linkRepository) GetIDsByUserID(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
    var ids []uuid.UUID
    err := r.db.WithContext(ctx).Model(&entity.Link{}).
        Where("user_id = ?", userID).
        Pluck("id", &ids).Error
    return ids, err
}