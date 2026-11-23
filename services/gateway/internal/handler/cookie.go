package handler

import (
	"net/http"
	"time"

	"github.com/deaprima/linkforge/services/gateway/internal/middleware"
	"github.com/gin-gonic/gin"
)

const (
	refreshTokenCookie = "refresh_token"
	oauthStateCookie   = "oauth_state"

	// Mencakup /auth/refresh dan /auth/logout (lihat API-CONTRACT.md bagian OAuth — Google)
	refreshCookiePath = "/api/v1/auth"
	oauthCookiePath   = "/api/v1/auth/oauth/google"
)

// CookieConfig mengatur atribut cookie token untuk client berbasis browser.
type CookieConfig struct {
	Secure          bool
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
}

func (cc CookieConfig) set(c *gin.Context, name, value, path string, ttl time.Duration) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     path,
		MaxAge:   int(ttl.Seconds()),
		HttpOnly: true,
		Secure:   cc.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (cc CookieConfig) clear(c *gin.Context, name, path string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     path,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   cc.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (cc CookieConfig) setTokens(c *gin.Context, accessToken, refreshToken string) {
	cc.set(c, middleware.AccessTokenCookie, accessToken, "/", cc.AccessTokenTTL)
	cc.set(c, refreshTokenCookie, refreshToken, refreshCookiePath, cc.RefreshTokenTTL)
}

func (cc CookieConfig) clearTokens(c *gin.Context) {
	cc.clear(c, middleware.AccessTokenCookie, "/")
	cc.clear(c, refreshTokenCookie, refreshCookiePath)
}
