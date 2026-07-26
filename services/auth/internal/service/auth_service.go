package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/deaprima/linkforge/services/auth/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/api/idtoken"
)

type RegisterInput struct {
	Name     string
	Email    string
	Password string
}

type LoginInput struct {
	Email    string
	Password string
}

type TokenResult struct {
	AccessToken  string
	RefreshToken string
}

type AuthService interface {
	Register(ctx context.Context, input RegisterInput) (*entity.User, *TokenResult, error)
	Login(ctx context.Context, input LoginInput) (*entity.User, *TokenResult, error)
	ValidateToken(ctx context.Context, tokenStr string) (*UserClaims, error)
	RefreshToken(ctx context.Context, refreshToken string, ip string, userAgent string) (*TokenResult, error)
	Logout(ctx context.Context, refreshToken string) error
	GetUser(ctx context.Context, userID uuid.UUID) (*entity.User, error)

	// API Key Management
	CreateApiKey(ctx context.Context, userID uuid.UUID, name string) (*entity.ApiKey, string, error)
	ListApiKeys(ctx context.Context, userID uuid.UUID) ([]entity.ApiKey, error)
	DeleteApiKey(ctx context.Context, userID uuid.UUID, keyID uuid.UUID) error
	ValidateApiKey(ctx context.Context, rawKey string) (*entity.ApiKey, error)

	// Google OAuth
    GoogleAuth(ctx context.Context, idToken string) (*entity.User, *TokenResult, error)
}

type authService struct {
	userRepo     repository.UserRepository
	tokenRepo    repository.TokenRepository
	keyRepo		 repository.ApiKeyRepository
	oauthRepo    repository.OAuthRepository
	googleClientID string
	tokenManager TokenManager
	refreshDur   time.Duration
}

func NewAuthService(
	userRepo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	keyRepo repository.ApiKeyRepository,
	oauthRepo repository.OAuthRepository,
	googleClientID string,
	tokenManager TokenManager,
	refreshDur time.Duration,
) AuthService {
	return &authService{
		userRepo:     userRepo,
		tokenRepo:    tokenRepo,
		keyRepo: 	  keyRepo,
		oauthRepo:    oauthRepo,	
		googleClientID: googleClientID,
		tokenManager: tokenManager,
		refreshDur:   refreshDur,
	}
}

func (s *authService) Register(ctx context.Context, input RegisterInput)  (*entity.User, *TokenResult, error) {
	
	// validasi email
	existingUser, err := s.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
   		return nil, nil, err
	}
	if existingUser != nil {
    	return nil, nil, errors.New("email already exists") 
	}

	// hashing
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
    	return nil, nil, err
	}
	pwdHash := string(hashedBytes)

	// save new user
	newUser := &entity.User{
		Name:			input.Name,
		Email:			input.Email,
		PasswordHash: 	&pwdHash,
	}
	if err:= s.userRepo.Create(ctx, newUser); err != nil {
		return nil, nil, err
	}

	// generate token
	tokens, err := s.generateAndSaveTokens(ctx, newUser.ID, newUser.Email, "", "")
	if err != nil {
		return nil, nil, err
	}
	return newUser, tokens, nil
}

func (s *authService) Login(ctx context.Context, input LoginInput) (*entity.User, *TokenResult, error) {
	// cari by email
	user, err := s.userRepo.GetByEmail(ctx, input.Email)
	if err != nil {
		return nil, nil, err
	}
	if user == nil || user.PasswordHash == nil {
		return nil, nil, errors.New("invalid credentials")
	}
	// cek password
	err = bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(input.Password))
	if err != nil {
		return nil, nil, errors.New("invalid credentials")
	}
	// generate Token (Access & Refresh)
	tokens, err := s.generateAndSaveTokens(ctx, user.ID, user.Email, "", "")
	if err != nil {
		return nil, nil, err
	}
	return user, tokens, nil
}

// helper untuk hash token secara deterministik (SHA-256)
func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (s *authService) generateAndSaveTokens(
	ctx context.Context,
	userID uuid.UUID,
	email string,
	ip string,
	userAgent string,
) (*TokenResult, error) {
	// Generate JWT Access Token
	accessToken, err := s.tokenManager.GenerateAccessToken(userID, email)
	if err != nil {
		return nil, err
	}
	// Generate Refresh Token acak
	rawRefreshToken, err := s.tokenManager.GenerateRefreshToken()
	if err != nil {
		return nil, err
	}
	
	// Gunakan SHA-256 untuk hash token agar bisa dicari secara O(1) di DB
	hashedToken := hashToken(rawRefreshToken)

	// Simpan ke database
	refreshTokenEntity := &entity.RefreshToken{
		UserID:    userID,
		TokenHash: hashedToken,
		ExpiresAt: time.Now().Add(s.refreshDur),
		UserAgent: userAgent,
		IPAddress: ip,
	}
	if err := s.tokenRepo.Create(ctx, refreshTokenEntity); err != nil {
		return nil, err
	}
	return &TokenResult{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken, // dikirim mentah ke client
	}, nil
}

func (s *authService) ValidateToken(ctx context.Context, tokenStr string) (*UserClaims, error) {
	return s.tokenManager.ValidateAccessToken(tokenStr)
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string, ip string, userAgent string) (*TokenResult, error) {
	hashed := hashToken(refreshToken)

	// 1. Cari token di database
	tokenEntity, err := s.tokenRepo.GetByHash(ctx, hashed)
	if err != nil {
		return nil, err
	}
	if tokenEntity == nil {
		return nil, errors.New("invalid refresh token")
	}

	// 2. Reuse Detection (Deteksi Penggunaan Kembali Token yang Sudah Mati)
	// Jika token sudah di-revoke sebelumnya, ini indikasi bahwa token pernah dicuri.
	// Tindakan pengamanan: Hapus/revoke seluruh active refresh token milik user tersebut!
	if tokenEntity.RevokedAt != nil {
		_ = s.tokenRepo.RevokeAllByUserID(ctx, tokenEntity.UserID)
		return nil, errors.New("refresh token abuse detected; all sessions revoked")
	}

	// 3. Periksa Expiration
	if time.Now().After(tokenEntity.ExpiresAt) {
		return nil, errors.New("refresh token expired")
	}

	// 4. Token Rotation (Revoke token saat ini)
	if err := s.tokenRepo.Revoke(ctx, tokenEntity.ID); err != nil {
		return nil, err
	}

	// 5. Ambil data User untuk generate token baru
	user, err := s.userRepo.GetByID(ctx, tokenEntity.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	// 6. Generate dan simpan pasangan token baru
	return s.generateAndSaveTokens(ctx, user.ID, user.Email, ip, userAgent)
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	hashed := hashToken(refreshToken)

	tokenEntity, err := s.tokenRepo.GetByHash(ctx, hashed)
	if err != nil {
		return err
	}
	if tokenEntity == nil {
		return errors.New("invalid refresh token")
	}

	// Revoke token agar tidak bisa digunakan lagi
	return s.tokenRepo.Revoke(ctx, tokenEntity.ID)
}

func (s *authService) GetUser(ctx context.Context, userID uuid.UUID) (*entity.User, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	return user, nil
}

// generateRawApiKey menghasilkan random key 32 byte dengan prefix "lf_sk_".
func generateRawApiKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("failed to generate api key: %w", err)
	}
	return "lf_sk_" + hex.EncodeToString(b), nil
}

func (s *authService) CreateApiKey(ctx context.Context, userID uuid.UUID, name string) (*entity.ApiKey, string, error) {
	if name == "" {
		return nil, "", errors.New("api key name cannot be empty")
	}

	rawKey, err := generateRawApiKey()
	if err != nil {
		return nil, "", err
	}

	h := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(h[:])

	// Simpan 12 char pertama sebagai prefix untuk ditampilkan di UI
	keyPrefix := rawKey[:12]

	key := &entity.ApiKey{
		UserID:    userID,
		Name:      name,
		KeyHash:   keyHash,
		KeyPrefix: keyPrefix,
	}

	if err := s.keyRepo.Create(ctx, key); err != nil {
		return nil, "", fmt.Errorf("failed to save api key: %w", err)
	}

	// rawKey HANYA dikembalikan di sini, tidak pernah disimpan
	return key, rawKey, nil
}

func (s *authService) ListApiKeys(ctx context.Context, userID uuid.UUID) ([]entity.ApiKey, error) {
	return s.keyRepo.ListByUserID(ctx, userID)
}

func (s *authService) DeleteApiKey(ctx context.Context, userID uuid.UUID, keyID uuid.UUID) error {
	return s.keyRepo.Revoke(ctx, keyID, userID)
}

func (s *authService) ValidateApiKey(ctx context.Context, rawKey string) (*entity.ApiKey, error) {
	h := sha256.Sum256([]byte(rawKey))
	keyHash := hex.EncodeToString(h[:])

	key, err := s.keyRepo.GetByHash(ctx, keyHash)
	if err != nil {
		return nil, fmt.Errorf("failed to validate api key: %w", err)
	}
	if key == nil {
		return nil, errors.New("invalid or revoked api key")
	}

	// Update last_used_at secara fire-and-forget, tidak block response
	go func() {
		_ = s.keyRepo.UpdateLastUsed(context.Background(), key.ID)
	}()

	return key, nil
}

// GoogleAuth memverifikasi Google ID Token, lalu login/register user secara otomatis.
func (s *authService) GoogleAuth(ctx context.Context, rawIDToken string) (*entity.User, *TokenResult, error) {
    // 1. Verifikasi ID Token ke Google
    payload, err := idtoken.Validate(ctx, rawIDToken, s.googleClientID) 
    if err != nil {
        return nil, nil, fmt.Errorf("invalid google id token: %w", err)
    }

    // 2. Ekstrak info user dari claims Google
    providerUserID := payload.Subject // Google UID (sub)
    email, _ := payload.Claims["email"].(string)
    name, _ := payload.Claims["name"].(string)

    if email == "" {
        return nil, nil, errors.New("google account does not provide email")
    }

    // 3. Cek apakah oauth_account sudah terdaftar
    oauthAccount, err := s.oauthRepo.GetByProviderAndUserID(ctx, "google", providerUserID)
    if err != nil {
        return nil, nil, err
    }

    var user *entity.User

    if oauthAccount != nil {
        // 3a. Sudah ada → langsung ambil user
        user, err = s.userRepo.GetByID(ctx, oauthAccount.UserID)
        if err != nil || user == nil {
            return nil, nil, errors.New("linked user not found")
        }
    } else {
        // 3b. Belum ada → cek apakah email sudah terdaftar (account linking by email)
        user, err = s.userRepo.GetByEmail(ctx, email)
        if err != nil {
            return nil, nil, err
        }

        if user == nil {
            // Buat user baru (OAuth-only, tanpa password)
            user = &entity.User{
                Email: email,
                Name:  name,
                // PasswordHash tetap nil — user OAuth-only
            }
            if err := s.userRepo.Create(ctx, user); err != nil {
                return nil, nil, fmt.Errorf("failed to create user: %w", err)
            }
        }

        // Buat oauth_account baru (link ke user yang ada atau baru dibuat)
        newOAuthAccount := &entity.OAuthAccount{
            UserID:         user.ID,
            Provider:       "google",
            ProviderUserID: providerUserID,
            Email:          email,
        }
        if err := s.oauthRepo.Create(ctx, newOAuthAccount); err != nil {
            return nil, nil, fmt.Errorf("failed to link google account: %w", err)
        }
    }

    // 4. Generate token pair
    tokens, err := s.generateAndSaveTokens(ctx, user.ID, user.Email, "", "")
    if err != nil {
        return nil, nil, err
    }

    return user, tokens, nil
}
