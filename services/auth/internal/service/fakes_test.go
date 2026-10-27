package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/google/uuid"
)

var errDB = errors.New("db is down")

// Implementasi in-memory dari repository.* untuk unit test.

type fakeUserRepo struct {
	mu        sync.Mutex
	users     map[uuid.UUID]*entity.User
	createErr error
	getErr    error
}

func newFakeUserRepo(users ...*entity.User) *fakeUserRepo {
	r := &fakeUserRepo{users: map[uuid.UUID]*entity.User{}}
	for _, u := range users {
		r.users[u.ID] = u
	}
	return r
}

func (r *fakeUserRepo) Create(_ context.Context, user *entity.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	user.ID = uuid.New()
	user.CreatedAt = time.Now()
	r.users[user.ID] = user
	return nil
}

func (r *fakeUserRepo) GetByID(_ context.Context, id uuid.UUID) (*entity.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	return r.users[id], nil
}

func (r *fakeUserRepo) GetByEmail(_ context.Context, email string) (*entity.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	for _, u := range r.users {
		if u.Email == email { // exact match, seperti unique index Postgres
			return u, nil
		}
	}
	return nil, nil
}

type fakeTokenRepo struct {
	mu        sync.Mutex
	tokens    map[uuid.UUID]*entity.RefreshToken
	createErr error
	revokeErr error
	revokeAll []uuid.UUID
}

func newFakeTokenRepo() *fakeTokenRepo {
	return &fakeTokenRepo{tokens: map[uuid.UUID]*entity.RefreshToken{}}
}

func (r *fakeTokenRepo) Create(_ context.Context, t *entity.RefreshToken) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	t.ID = uuid.New()
	r.tokens[t.ID] = t
	return nil
}

// GetByHash juga mengembalikan token yang sudah di-revoke (dibutuhkan reuse detection)
func (r *fakeTokenRepo) GetByHash(_ context.Context, hash string) (*entity.RefreshToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, t := range r.tokens {
		if t.TokenHash == hash {
			return t, nil
		}
	}
	return nil, nil
}

func (r *fakeTokenRepo) Revoke(_ context.Context, id uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.revokeErr != nil {
		return r.revokeErr
	}
	now := time.Now()
	r.tokens[id].RevokedAt = &now
	return nil
}

func (r *fakeTokenRepo) RevokeAllByUserID(_ context.Context, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revokeAll = append(r.revokeAll, userID)
	now := time.Now()
	for _, t := range r.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			t.RevokedAt = &now
		}
	}
	return nil
}

// active mengembalikan token milik user yang belum di-revoke.
func (r *fakeTokenRepo) active(userID uuid.UUID) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, t := range r.tokens {
		if t.UserID == userID && t.RevokedAt == nil {
			n++
		}
	}
	return n
}

type fakeKeyRepo struct {
	mu        sync.Mutex
	keys      map[uuid.UUID]*entity.ApiKey
	createErr error
	getErr    error
	usedCh    chan uuid.UUID
}

func newFakeKeyRepo() *fakeKeyRepo {
	return &fakeKeyRepo{keys: map[uuid.UUID]*entity.ApiKey{}, usedCh: make(chan uuid.UUID, 10)}
}

func (r *fakeKeyRepo) Create(_ context.Context, k *entity.ApiKey) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	k.ID = uuid.New()
	k.CreatedAt = time.Now()
	r.keys[k.ID] = k
	return nil
}

func (r *fakeKeyRepo) GetByID(_ context.Context, id uuid.UUID) (*entity.ApiKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.keys[id], nil
}

// GetByHash hanya mengembalikan key yang belum di-revoke (sama seperti repository asli)
func (r *fakeKeyRepo) GetByHash(_ context.Context, hash string) (*entity.ApiKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getErr != nil {
		return nil, r.getErr
	}
	for _, k := range r.keys {
		if k.KeyHash == hash && k.RevokedAt == nil {
			return k, nil
		}
	}
	return nil, nil
}

func (r *fakeKeyRepo) ListByUserID(_ context.Context, userID uuid.UUID) ([]entity.ApiKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []entity.ApiKey
	for _, k := range r.keys {
		if k.UserID == userID && k.RevokedAt == nil {
			out = append(out, *k)
		}
	}
	return out, nil
}

func (r *fakeKeyRepo) Revoke(_ context.Context, id, userID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k, ok := r.keys[id]
	if !ok || k.UserID != userID || k.RevokedAt != nil {
		return errors.New("api key not found")
	}
	now := time.Now()
	k.RevokedAt = &now
	return nil
}

func (r *fakeKeyRepo) UpdateLastUsed(_ context.Context, id uuid.UUID) error {
	r.usedCh <- id
	return nil
}

type fakeOAuthRepo struct {
	mu        sync.Mutex
	accounts  []*entity.OAuthAccount
	createErr error
}

func (r *fakeOAuthRepo) GetByProviderAndUserID(_ context.Context, provider, providerUserID string) (*entity.OAuthAccount, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, a := range r.accounts {
		if a.Provider == provider && a.ProviderUserID == providerUserID {
			return a, nil
		}
	}
	return nil, nil
}

func (r *fakeOAuthRepo) Create(_ context.Context, a *entity.OAuthAccount) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.createErr != nil {
		return r.createErr
	}
	a.ID = uuid.New()
	r.accounts = append(r.accounts, a)
	return nil
}

func ptr[T any](v T) *T { return &v }
