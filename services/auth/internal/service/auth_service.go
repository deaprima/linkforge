package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/deaprima/linkforge/services/auth/internal/entity"
	"github.com/deaprima/linkforge/services/auth/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
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
}

type authService struct {
	userRepo     repository.UserRepository
	tokenRepo    repository.TokenRepository
	tokenManager TokenManager
	refreshDur   time.Duration
}

func NewAuthService(
	userRepo repository.UserRepository,
	tokenRepo repository.TokenRepository,
	tokenManager TokenManager,
	refreshDur time.Duration,
) AuthService {
	return &authService{
		userRepo:     userRepo,
		tokenRepo:    tokenRepo,
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


