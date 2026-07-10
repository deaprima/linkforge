package service

import (
	"context"
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

	// TODO Refresh token
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
	// Enkripsi/Hash refresh token sebelum masuk DB (Bcrypt)
	hashedToken, err := bcrypt.GenerateFromPassword([]byte(rawRefreshToken), bcrypt.MinCost)
	if err != nil {
		return nil, err
	}
	// Simpan ke database
	refreshTokenEntity := &entity.RefreshToken{
		UserID:    userID,
		TokenHash: string(hashedToken),
		ExpiresAt: time.Now().Add(s.refreshDur),
		UserAgent: userAgent,
		IPAddress: ip,
	}
	if err := s.tokenRepo.Create(ctx, refreshTokenEntity); err != nil {
		return nil, err
	}
	return &TokenResult{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken, // Yang dikirim ke user adalah TOKEN MENTAH (raw)
	}, nil
}
