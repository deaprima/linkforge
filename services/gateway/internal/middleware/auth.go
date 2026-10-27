package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/deaprima/linkforge/services/gateway/internal/response"
	pb "github.com/deaprima/linkforge/services/shared/proto/auth"
	"github.com/gin-gonic/gin"
)

const (
	ctxUserID = "user_id"

	// AccessTokenCookie diset oleh OAuth callback; dipakai sebagai pengganti header Bearer
	// untuk client berbasis browser yang tidak bisa membaca HttpOnly cookie.
	AccessTokenCookie = "access_token"
)

// CredentialValidator adalah subset AuthClient yang dibutuhkan middleware.
type CredentialValidator interface {
	ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error)
	ValidateApiKey(ctx context.Context, req *pb.ValidateApiKeyRequest) (*pb.ValidateApiKeyResponse, error)
}

// RequireAuth memvalidasi Bearer JWT (header Authorization atau cookie access_token).
// Kalau allowAPIKey true, header X-API-Key juga diterima — hanya untuk endpoint link,
// bukan manajemen akun (lihat API-CONTRACT.md GET /auth/me).
func RequireAuth(v CredentialValidator, allowAPIKey bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()

		if token, ok := bearerToken(c); ok {
			res, err := v.ValidateToken(ctx, &pb.ValidateTokenRequest{AccessToken: token})
			if err != nil {
				response.GRPCError(c, err, nil)
				return
			}
			if !res.GetIsValid() {
				response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "Invalid or expired access token.")
				return
			}
			c.Set(ctxUserID, res.GetUserId())
			c.Next()
			return
		}

		if key := c.GetHeader("X-API-Key"); key != "" {
			if !allowAPIKey {
				response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "This endpoint requires a Bearer token; API keys are not accepted.")
				return
			}
			res, err := v.ValidateApiKey(ctx, &pb.ValidateApiKeyRequest{ApiKey: key})
			if err != nil {
				response.GRPCError(c, err, nil)
				return
			}
			if !res.GetIsValid() {
				response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "Invalid or revoked API key.")
				return
			}
			c.Set(ctxUserID, res.GetUserId())
			c.Next()
			return
		}

		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "Authentication required.")
	}
}

// UserID mengambil user_id yang diset RequireAuth.
func UserID(c *gin.Context) string {
	return c.GetString(ctxUserID)
}

func bearerToken(c *gin.Context) (string, bool) {
	if header := c.GetHeader("Authorization"); header != "" {
		token, found := strings.CutPrefix(header, "Bearer ")
		token = strings.TrimSpace(token)
		return token, found && token != ""
	}
	if cookie, err := c.Cookie(AccessTokenCookie); err == nil && cookie != "" {
		return cookie, true
	}
	return "", false
}
