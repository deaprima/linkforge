package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/deaprima/linkforge/services/link/internal/entity"
	"github.com/deaprima/linkforge/services/link/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type CreateLinkInput struct {
    UserID      uuid.UUID
    OriginalURL string
    Alias       string     
    ExpiredAt   *time.Time 
    Password    string     
}

type UpdateLinkInput struct {
    ID          uuid.UUID
    UserID      uuid.UUID
    OriginalURL string     
    Alias       string     
    ExpiredAt   *time.Time 
    ClearExpiry bool       
}

type ResolveResult struct {
    LinkID      uuid.UUID
    OriginalURL string
    IsExpired   bool
    HasPassword bool
}

type LinkService interface {
    CreateLink(ctx context.Context, input CreateLinkInput) (*LinkView, error)
    GetLink(ctx context.Context, id, userID uuid.UUID) (*LinkView, error)
    ListLinks(ctx context.Context, userID uuid.UUID, page, limit int) ([]*LinkView, int64, error)
    UpdateLink(ctx context.Context, input UpdateLinkInput) (*LinkView, error)
    DeleteLink(ctx context.Context, id, userID uuid.UUID) error
    ResolveAlias(ctx context.Context, alias string) (*ResolveResult, error)
    VerifyLinkPassword(ctx context.Context, alias, password string) (*LinkView, error)
    GetUserLinkIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
}

type LinkView struct {
    ID          uuid.UUID
    UserID      uuid.UUID
    OriginalURL string
    Alias       string    
    ClickCount  int64
    HasPassword bool
    ExpiredAt   *time.Time
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
func toLinkView(link *entity.Link) *LinkView {
    alias := link.ShortCode
    if link.CustomAlias != nil && *link.CustomAlias != "" {
        alias = *link.CustomAlias
    }
    return &LinkView{
        ID:          link.ID,
        UserID:      link.UserID,
        OriginalURL: link.OriginalURL,
        Alias:       alias,
        ClickCount:  link.ClickCount,
        HasPassword: link.PasswordHash != nil,
        ExpiredAt:   link.ExpiredAt,
        CreatedAt:   link.CreatedAt,
        UpdatedAt:   link.UpdatedAt,
    }
}

type linkService struct {
    linkRepo   repository.LinkRepository
    domainRepo repository.DomainRepository
}

func NewLinkService(
    linkRepo repository.LinkRepository,
    domainRepo repository.DomainRepository,
) LinkService {
    return &linkService{
        linkRepo:   linkRepo,
        domainRepo: domainRepo,
    }
}

// extractDomain mengambil host dari sebuah URL string.
func extractDomain(rawURL string) (string, error) {
    parsed, err := url.Parse(rawURL)
    if err != nil || parsed.Host == "" {
        return "", fmt.Errorf("invalid URL: %s", rawURL)
    }
    host := parsed.Hostname()
    return strings.ToLower(host), nil
}

func (s *linkService) CreateLink(ctx context.Context, input CreateLinkInput) (*LinkView, error) {
    // 1. Validasi URL (harus valid & tidak kosong)
    if strings.TrimSpace(input.OriginalURL) == "" {
        return nil, errors.New("original_url cannot be empty")
    }

    // 2. Malicious URL check — ekstrak domain, cek blocked_domains
    domain, err := extractDomain(input.OriginalURL)
    if err != nil {
        return nil, fmt.Errorf("invalid url: %w", err)
    }
    blocked, err := s.domainRepo.IsBlocked(ctx, domain)
    if err != nil {
        return nil, fmt.Errorf("failed to check blocked domain: %w", err)
    }
    if blocked {
        return nil, errors.New("url domain is blocked")
    }

    link := &entity.Link{
        UserID:      input.UserID,
        OriginalURL: input.OriginalURL,
        IsActive:    true,
        ExpiredAt:   input.ExpiredAt,
    }

    // 3. Tentukan alias: custom atau auto-generate short_code
	if input.Alias != "" {
	    // Validate DULU sebelum generate short code
	    if err := validateCustomAlias(input.Alias); err != nil {
	        return nil, err
	    }
	    link.CustomAlias = &input.Alias
	    sc, err := s.generateUniqueShortCode(ctx)
	    if err != nil {
	        return nil, err
	    }
	    link.ShortCode = sc
	} else {
	    sc, err := s.generateUniqueShortCode(ctx)
	    if err != nil {
	        return nil, err
	    }
	    link.ShortCode = sc
	}

    // 4. Password hash
	if input.Password != "" {
	    hashed, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	    if err != nil {
	        return nil, fmt.Errorf("failed to hash password: %w", err)
	    }
	    h := string(hashed)       // ← ganti nama dari 's' ke 'h'
	    link.PasswordHash = &h
	}


    if err := s.linkRepo.Create(ctx, link); err != nil {
        return nil, fmt.Errorf("failed to create link: %w", err)
    }
    return toLinkView(link), nil
}

// generateUniqueShortCode membuat short_code dan retry jika collision.
func (s *linkService) generateUniqueShortCode(ctx context.Context) (string, error) {
    for i := 0; i < 5; i++ { // max 5 attempt
        code, err := generateShortCode()
        if err != nil {
            return "", err
        }
        existing, err := s.linkRepo.GetByShortCode(ctx, code)
        if err != nil {
            return "", err
        }
        if existing == nil {
            return code, nil // tidak ada collision
        }
    }
    return "", errors.New("failed to generate unique short code after retries")
}

func (s *linkService) GetLink(ctx context.Context, id, userID uuid.UUID) (*LinkView, error) {
    link, err := s.linkRepo.GetByIDAndUserID(ctx, id, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to get link: %w", err)
    }
    if link == nil {
        return nil, errors.New("link not found")
    }
    return toLinkView(link), nil
}

func (s *linkService) ListLinks(ctx context.Context, userID uuid.UUID, page, limit int) ([]*LinkView, int64, error) {
    if page < 1 { page = 1 }
    if limit < 1 || limit > 100 { limit = 20 }

    links, err := s.linkRepo.ListByUserID(ctx, userID, page, limit)
    if err != nil {
        return nil, 0, fmt.Errorf("failed to list links: %w", err)
    }
    total, err := s.linkRepo.CountByUserID(ctx, userID)
    if err != nil {
        return nil, 0, fmt.Errorf("failed to count links: %w", err)
    }

    views := make([]*LinkView, len(links))
    for i := range links {
        views[i] = toLinkView(&links[i])
    }
    return views, total, nil
}

func (s *linkService) UpdateLink(ctx context.Context, input UpdateLinkInput) (*LinkView, error) {
    link, err := s.linkRepo.GetByIDAndUserID(ctx, input.ID, input.UserID)
    if err != nil {
        return nil, fmt.Errorf("failed to get link: %w", err)
    }
    if link == nil {
        return nil, errors.New("link not found")
    }

    // Update original_url jika disediakan
    if input.OriginalURL != "" {
        domain, err := extractDomain(input.OriginalURL)
        if err != nil {
            return nil, fmt.Errorf("invalid url: %w", err)
        }
        blocked, err := s.domainRepo.IsBlocked(ctx, domain)
        if err != nil {
            return nil, err
        }
        if blocked {
            return nil, errors.New("url domain is blocked")
        }
        link.OriginalURL = input.OriginalURL
    }

    // Update alias jika disediakan
    if input.Alias != "" {
        link.CustomAlias = &input.Alias
    }

    // Update expiry
    if input.ClearExpiry {
        link.ExpiredAt = nil
    } else if input.ExpiredAt != nil {
        link.ExpiredAt = input.ExpiredAt
    }

    if err := s.linkRepo.Update(ctx, link); err != nil {
        return nil, fmt.Errorf("failed to update link: %w", err)
    }
    return toLinkView(link), nil
}

func (s *linkService) DeleteLink(ctx context.Context, id, userID uuid.UUID) error {
    err := s.linkRepo.SoftDelete(ctx, id, userID)
    if err != nil {
        return fmt.Errorf("failed to delete link: %w", err)
    }
    return nil
}

func (s *linkService) ResolveAlias(ctx context.Context, alias string) (*ResolveResult, error) {
    // Coba cari via custom_alias dulu
    link, err := s.linkRepo.GetByCustomAlias(ctx, alias)
    if err != nil {
        return nil, fmt.Errorf("failed to resolve alias: %w", err)
    }

    // Jika tidak ketemu via custom_alias, coba via short_code
    if link == nil {
        link, err = s.linkRepo.GetByShortCode(ctx, alias)
        if err != nil {
            return nil, fmt.Errorf("failed to resolve alias: %w", err)
        }
    }

    if link == nil {
        return nil, errors.New("link not found")
    }

    // Cek expired
    isExpired := false
    if link.ExpiredAt != nil && time.Now().After(*link.ExpiredAt) {
        isExpired = true
    }

    // Increment click count secara async (fire-and-forget)
    if !isExpired {
        go func() {
            _ = s.linkRepo.IncrementClickCount(context.Background(), link.ID)
        }()
    }

    return &ResolveResult{
        LinkID:      link.ID,
        OriginalURL: link.OriginalURL,
        IsExpired:   isExpired,
        HasPassword: link.PasswordHash != nil,
    }, nil
}

func (s *linkService) VerifyLinkPassword(ctx context.Context, alias, password string) (*LinkView, error) {
    // Resolve alias dulu
    link, err := s.linkRepo.GetByCustomAlias(ctx, alias)
    if err != nil {
        return nil, err
    }
    if link == nil {
        link, err = s.linkRepo.GetByShortCode(ctx, alias)
        if err != nil {
            return nil, err
        }
    }
    if link == nil {
        return nil, errors.New("link not found")
    }
    if link.PasswordHash == nil {
        return nil, errors.New("link is not password protected")
    }
    if err := bcrypt.CompareHashAndPassword([]byte(*link.PasswordHash), []byte(password)); err != nil {
        return nil, errors.New("invalid password")
    }
    return toLinkView(link), nil
}

func (s *linkService) GetUserLinkIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
    return s.linkRepo.GetIDsByUserID(ctx, userID)
}

var aliasRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{3,64}$`)
func validateCustomAlias(alias string) error {
    if !aliasRegex.MatchString(alias) {
        return errors.New("alias hanya boleh huruf, angka, tanda hubung (-) dan underscore (_), panjang 3-64 karakter")
    }
    // Reserved words yang tidak boleh dipakai sebagai alias
    reserved := []string{"api", "admin", "login", "register", "health", "metrics"}
    for _, r := range reserved {
        if strings.EqualFold(alias, r) {
            return fmt.Errorf("alias '%s' is reserved", alias)
        }
    }
    return nil
}
