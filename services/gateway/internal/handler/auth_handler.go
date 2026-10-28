package handler

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/deaprima/linkforge/services/gateway/internal/middleware"
	"github.com/deaprima/linkforge/services/gateway/internal/response"
	pb "github.com/deaprima/linkforge/services/shared/proto/auth"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
)

// AuthService adalah subset AuthClient yang dibutuhkan AuthHandler (memudahkan mocking di test).
type AuthService interface {
	Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error)
	Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error)
	RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error)
	Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error)
	GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.GetUserResponse, error)
	CreateApiKey(ctx context.Context, req *pb.CreateApiKeyRequest) (*pb.CreateApiKeyResponse, error)
	ListApiKeys(ctx context.Context, req *pb.ListApiKeysRequest) (*pb.ListApiKeysResponse, error)
	DeleteApiKey(ctx context.Context, req *pb.DeleteApiKeyRequest) (*pb.DeleteApiKeyResponse, error)
	GoogleAuth(ctx context.Context, req *pb.GoogleAuthRequest) (*pb.GoogleAuthResponse, error)
	RequestPasswordReset(ctx context.Context, req *pb.RequestPasswordResetRequest) (*pb.RequestPasswordResetResponse, error)
	ResetPassword(ctx context.Context, req *pb.ResetPasswordRequest) (*pb.ResetPasswordResponse, error)
}

type AuthHandler struct {
	auth        AuthService
	google      *GoogleOAuth
	cookies     CookieConfig
	frontendURL string
}

func NewAuthHandler(auth AuthService, google *GoogleOAuth, cookies CookieConfig, frontendURL string) *AuthHandler {
	return &AuthHandler{auth: auth, google: google, cookies: cookies, frontendURL: frontendURL}
}

// DTO — bentuk response publik sesuai API-CONTRACT.md

type userDTO struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Email     string     `json:"email"`
	CreatedAt *time.Time `json:"created_at,omitempty"`
}

type tokenDTO struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type authDTO struct {
	User userDTO `json:"user"`
	tokenDTO
}

type apiKeyDTO struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	LastUsedAt *time.Time `json:"last_used_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

type createdApiKeyDTO struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserDTO(u *pb.User) userDTO {
	return userDTO{
		ID:        u.GetId(),
		Name:      u.GetName(),
		Email:     u.GetEmail(),
		CreatedAt: timePtr(u.GetCreatedAt()),
	}
}

func toAuthDTO(u *pb.User, t *pb.TokenPair) authDTO {
	return authDTO{
		User:     toUserDTO(u),
		tokenDTO: tokenDTO{AccessToken: t.GetAccessToken(), RefreshToken: t.GetRefreshToken()},
	}
}

// Request body

type registerRequest struct {
	Name     string `json:"name" binding:"required,max=100"`
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8,max=72"` // bcrypt hanya membaca 72 byte pertama
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type refreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type forgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email,max=255"`
}

type resetPasswordRequest struct {
	Token    string `json:"token" binding:"required,max=256"`
	Password string `json:"password" binding:"required,min=8,max=72"` // bcrypt hanya membaca 72 byte pertama
}

type createApiKeyRequest struct {
	Name string `json:"name" binding:"required,max=100"`
}

// Handlers

// POST /api/v1/auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if !bindJSON(c, &req, false) {
		return
	}

	res, err := h.auth.Register(rpcContext(c), &pb.RegisterRequest{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		response.GRPCError(c, err, response.Overrides{
			codes.AlreadyExists: {Status: http.StatusConflict, Code: response.CodeEmailExists},
		})
		return
	}
	h.respondSession(c, http.StatusCreated, res.GetUser(), res.GetTokens())
}

// POST /api/v1/auth/forgot-password
// Balasan selalu sama baik email terdaftar maupun tidak, supaya endpoint ini tidak bisa dipakai
// untuk mengecek siapa yang punya akun.
func (h *AuthHandler) ForgotPassword(c *gin.Context) {
	var req forgotPasswordRequest
	if !bindJSON(c, &req, false) {
		return
	}
	if _, err := h.auth.RequestPasswordReset(rpcContext(c), &pb.RequestPasswordResetRequest{Email: req.Email}); err != nil {
		response.GRPCError(c, err, nil)
		return
	}
	response.OK(c, http.StatusOK, gin.H{"message": "If that email is registered, a reset link is on its way."})
}

// POST /api/v1/auth/reset-password
func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req resetPasswordRequest
	if !bindJSON(c, &req, false) {
		return
	}
	_, err := h.auth.ResetPassword(rpcContext(c), &pb.ResetPasswordRequest{Token: req.Token, NewPassword: req.Password})
	if err != nil {
		response.GRPCError(c, err, response.Overrides{
			codes.InvalidArgument: {Status: http.StatusBadRequest, Code: response.CodeInvalidResetToken},
		})
		return
	}
	// Semua refresh token user sudah dicabut; cookie sesi yang tersisa di browser ini tidak berguna lagi
	h.cookies.clearTokens(c)
	response.OK(c, http.StatusOK, gin.H{"message": "Your password has been changed. Sign in with the new password."})
}

// sessionModeHeader: web app mengirim "X-Session-Mode: cookie" supaya token tidak pernah ada di body
// JSON (yang bisa dibaca JavaScript/XSS). Client lain tetap menerima token di body.
const sessionModeHeader = "X-Session-Mode"

// respondSession memasang cookie sesi, lalu membalas user (+ token kecuali mode cookie-only).
func (h *AuthHandler) respondSession(c *gin.Context, status int, user *pb.User, tokens *pb.TokenPair) {
	h.cookies.setTokens(c, tokens.GetAccessToken(), tokens.GetRefreshToken())
	if c.GetHeader(sessionModeHeader) == "cookie" {
		response.OK(c, status, gin.H{"user": toUserDTO(user)})
		return
	}
	response.OK(c, status, toAuthDTO(user, tokens))
}

// POST /api/v1/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if !bindJSON(c, &req, false) {
		return
	}

	res, err := h.auth.Login(rpcContext(c), &pb.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		response.GRPCError(c, err, nil)
		return
	}
	h.respondSession(c, http.StatusOK, res.GetUser(), res.GetTokens())
}

// POST /api/v1/auth/refresh — refresh token dari body, atau dari cookie (client OAuth/browser)
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req refreshTokenRequest
	if !bindJSON(c, &req, true) {
		return
	}

	fromCookie := false
	if req.RefreshToken == "" {
		req.RefreshToken, _ = c.Cookie(refreshTokenCookie)
		fromCookie = true
	}
	if req.RefreshToken == "" {
		response.Error(c, http.StatusBadRequest, response.CodeValidation, "refresh_token is required")
		return
	}

	res, err := h.auth.RefreshToken(rpcContext(c), &pb.RefreshTokenRequest{RefreshToken: req.RefreshToken})
	if err != nil {
		if fromCookie {
			h.cookies.clearTokens(c)
		}
		// Reuse detection di Auth Service mengembalikan PermissionDenied — tetap 401 bagi client
		response.GRPCError(c, err, response.Overrides{
			codes.PermissionDenied: {Status: http.StatusUnauthorized, Code: response.CodeUnauthorized},
		})
		return
	}

	tokens := res.GetTokens()
	if fromCookie {
		// Sesi browser: token baru hanya lewat cookie, tidak pernah di body (lihat respondSession)
		h.cookies.setTokens(c, tokens.GetAccessToken(), tokens.GetRefreshToken())
		response.NoContent(c)
		return
	}
	response.OK(c, http.StatusOK, tokenDTO{
		AccessToken:  tokens.GetAccessToken(),
		RefreshToken: tokens.GetRefreshToken(),
	})
}

// POST /api/v1/auth/logout (Bearer)
func (h *AuthHandler) Logout(c *gin.Context) {
	var req refreshTokenRequest
	if !bindJSON(c, &req, true) {
		return
	}
	if req.RefreshToken == "" {
		req.RefreshToken, _ = c.Cookie(refreshTokenCookie)
	}
	if req.RefreshToken == "" {
		response.Error(c, http.StatusBadRequest, response.CodeValidation, "refresh_token is required")
		return
	}

	_, err := h.auth.Logout(rpcContext(c), &pb.LogoutRequest{RefreshToken: req.RefreshToken})
	h.cookies.clearTokens(c)
	if err != nil {
		response.GRPCError(c, err, nil)
		return
	}
	response.NoContent(c)
}

// GET /api/v1/auth/me (Bearer)
func (h *AuthHandler) Me(c *gin.Context) {
	res, err := h.auth.GetUser(rpcContext(c), &pb.GetUserRequest{UserId: middleware.UserID(c)})
	if err != nil {
		response.GRPCError(c, err, nil)
		return
	}
	response.OK(c, http.StatusOK, toUserDTO(res.GetUser()))
}

// POST /api/v1/auth/api-keys (Bearer)
func (h *AuthHandler) CreateApiKey(c *gin.Context) {
	var req createApiKeyRequest
	if !bindJSON(c, &req, false) {
		return
	}

	res, err := h.auth.CreateApiKey(rpcContext(c), &pb.CreateApiKeyRequest{
		UserId: middleware.UserID(c),
		Name:   req.Name,
	})
	if err != nil {
		response.GRPCError(c, err, nil)
		return
	}

	key := res.GetApiKey()
	response.OK(c, http.StatusCreated, createdApiKeyDTO{
		ID:        key.GetId(),
		Name:      key.GetName(),
		Key:       res.GetRawKey(),
		CreatedAt: timeVal(key.GetCreatedAt()),
	})
}

// GET /api/v1/auth/api-keys (Bearer)
func (h *AuthHandler) ListApiKeys(c *gin.Context) {
	res, err := h.auth.ListApiKeys(rpcContext(c), &pb.ListApiKeysRequest{UserId: middleware.UserID(c)})
	if err != nil {
		response.GRPCError(c, err, nil)
		return
	}

	keys := make([]apiKeyDTO, 0, len(res.GetApiKeys()))
	for _, k := range res.GetApiKeys() {
		keys = append(keys, apiKeyDTO{
			ID:         k.GetId(),
			Name:       k.GetName(),
			KeyPrefix:  k.GetKeyPrefix(),
			LastUsedAt: timePtr(k.GetLastUsedAt()),
			CreatedAt:  timeVal(k.GetCreatedAt()),
		})
	}
	response.OK(c, http.StatusOK, keys)
}

// DELETE /api/v1/auth/api-keys/:id (Bearer)
func (h *AuthHandler) DeleteApiKey(c *gin.Context) {
	_, err := h.auth.DeleteApiKey(rpcContext(c), &pb.DeleteApiKeyRequest{
		UserId:   middleware.UserID(c),
		ApiKeyId: c.Param("id"),
	})
	if err != nil {
		response.GRPCError(c, err, nil)
		return
	}
	response.NoContent(c)
}

// GET /api/v1/auth/oauth/google
func (h *AuthHandler) GoogleLogin(c *gin.Context) {
	if !h.google.Enabled() {
		response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "Google OAuth is not configured.")
		return
	}

	state, err := randomState()
	if err != nil {
		log.Printf("[gateway] failed to generate oauth state: %v", err)
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "An unexpected error occurred.")
		return
	}

	// State disimpan di cookie untuk proteksi CSRF, dicocokkan lagi di callback
	h.cookies.set(c, oauthStateCookie, state, oauthCookiePath, 10*time.Minute)
	c.Redirect(http.StatusFound, h.google.AuthCodeURL(state))
}

// GET /api/v1/auth/oauth/google/callback
func (h *AuthHandler) GoogleCallback(c *gin.Context) {
	if !h.google.Enabled() {
		response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "Google OAuth is not configured.")
		return
	}

	expectedState, _ := c.Cookie(oauthStateCookie)
	h.cookies.clear(c, oauthStateCookie, oauthCookiePath)

	if errParam := c.Query("error"); errParam != "" {
		h.redirectOAuthError(c, errParam)
		return
	}

	state, code := c.Query("state"), c.Query("code")
	if code == "" || state == "" || expectedState == "" ||
		subtle.ConstantTimeCompare([]byte(state), []byte(expectedState)) != 1 {
		h.redirectOAuthError(c, "invalid_state")
		return
	}

	idToken, err := h.google.ExchangeIDToken(c.Request.Context(), code)
	if err != nil {
		log.Printf("[gateway] google oauth exchange: %v", err)
		h.redirectOAuthError(c, "exchange_failed")
		return
	}

	res, err := h.auth.GoogleAuth(rpcContext(c), &pb.GoogleAuthRequest{IdToken: idToken})
	if err != nil {
		log.Printf("[gateway] google auth rpc: %v", err)
		h.redirectOAuthError(c, "auth_failed")
		return
	}

	h.cookies.setTokens(c, res.GetTokens().GetAccessToken(), res.GetTokens().GetRefreshToken())
	c.Redirect(http.StatusFound, h.frontendURL+WebPathDashboard)
}

func (h *AuthHandler) redirectOAuthError(c *gin.Context, reason string) {
	c.Redirect(http.StatusFound, h.frontendURL+WebPathLogin+"?error="+url.QueryEscape(reason))
}

func randomState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
