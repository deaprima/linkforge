package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/deaprima/linkforge/services/link/internal/entity"
	"github.com/google/uuid"
)

var errDB = errors.New("db is down")

// fakeLinkRepo adalah implementasi in-memory dari repository.LinkRepository untuk unit test.
type fakeLinkRepo struct {
	mu    sync.Mutex
	links map[uuid.UUID]*entity.Link

	// Error injection
	createErr error
	getErr    error
	listErr   error
	countErr  error
	updateErr error
	deleteErr error

	// shortCodeCollisions: jumlah panggilan GetByShortCode pertama yang dipaksa "sudah dipakai"
	shortCodeCollisions int
	shortCodeLookups    int

	listPage, listLimit int
	clicked             chan uuid.UUID
}

func newFakeLinkRepo(links ...*entity.Link) *fakeLinkRepo {
	r := &fakeLinkRepo{links: map[uuid.UUID]*entity.Link{}, clicked: make(chan uuid.UUID, 10)}
	for _, l := range links {
		r.links[l.ID] = l
	}
	return r
}

func (r *fakeLinkRepo) Create(_ context.Context, link *entity.Link) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	link.ID = uuid.New()
	link.CreatedAt = time.Now()
	link.UpdatedAt = link.CreatedAt
	r.links[link.ID] = link
	return nil
}

func (r *fakeLinkRepo) GetByID(_ context.Context, id uuid.UUID) (*entity.Link, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.links[id], nil
}

func (r *fakeLinkRepo) GetByIDAndUserID(_ context.Context, id, userID uuid.UUID) (*entity.Link, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	if l, ok := r.links[id]; ok && l.UserID == userID {
		return l, nil
	}
	return nil, nil
}

func (r *fakeLinkRepo) ListByUserID(_ context.Context, userID uuid.UUID, page, limit int) ([]entity.Link, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.listPage, r.listLimit = page, limit
	if r.listErr != nil {
		return nil, r.listErr
	}
	var out []entity.Link
	for _, l := range r.links {
		if l.UserID == userID {
			out = append(out, *l)
		}
	}
	return out, nil
}

func (r *fakeLinkRepo) CountByUserID(_ context.Context, userID uuid.UUID) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.countErr != nil {
		return 0, r.countErr
	}
	var n int64
	for _, l := range r.links {
		if l.UserID == userID {
			n++
		}
	}
	return n, nil
}

func (r *fakeLinkRepo) Update(_ context.Context, link *entity.Link) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.updateErr != nil {
		return r.updateErr
	}
	link.UpdatedAt = time.Now()
	r.links[link.ID] = link
	return nil
}

func (r *fakeLinkRepo) SoftDelete(_ context.Context, id, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.deleteErr != nil {
		return r.deleteErr
	}
	if l, ok := r.links[id]; ok && l.UserID == userID {
		delete(r.links, id)
		return nil
	}
	return errors.New("link not found or not owned by user")
}

func (r *fakeLinkRepo) GetByShortCode(_ context.Context, shortCode string) (*entity.Link, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	r.shortCodeLookups++
	if r.shortCodeLookups <= r.shortCodeCollisions {
		return &entity.Link{ShortCode: shortCode}, nil
	}
	for _, l := range r.links {
		if l.ShortCode == shortCode {
			return l, nil
		}
	}
	return nil, nil
}

func (r *fakeLinkRepo) GetByCustomAlias(_ context.Context, alias string) (*entity.Link, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	for _, l := range r.links {
		if l.CustomAlias != nil && *l.CustomAlias == alias {
			return l, nil
		}
	}
	return nil, nil
}

func (r *fakeLinkRepo) IncrementClickCount(_ context.Context, id uuid.UUID) error {
	r.clicked <- id
	return nil
}

func (r *fakeLinkRepo) GetIDsByUserID(_ context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var ids []uuid.UUID
	for id, l := range r.links {
		if l.UserID == userID {
			ids = append(ids, id)
		}
	}
	return ids, nil
}

// fakeDomainRepo mem-blokir domain yang ada di map.
type fakeDomainRepo struct {
	blocked map[string]bool
	err     error
	checked []string
}

func (r *fakeDomainRepo) IsBlocked(_ context.Context, domain string) (bool, error) {
	r.checked = append(r.checked, domain)
	if r.err != nil {
		return false, r.err
	}
	return r.blocked[domain], nil
}

func ptr[T any](v T) *T { return &v }
