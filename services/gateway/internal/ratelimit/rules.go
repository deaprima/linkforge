package ratelimit

import (
	"github.com/deaprima/linkforge/services/gateway/internal/middleware"
	"time"

	"github.com/gin-gonic/gin"
)

// maxAliasKeyLen membatasi panjang alias di key Redis (alias valid maks 64 karakter);
// alias acak yang lebih panjang tetap dibatasi rule per-IP, jadi tidak bisa membanjiri Redis.
const maxAliasKeyLen = 64

// VerifyPasswordRules = brute-force protection POST /:alias/verify (lihat API-CONTRACT.md):
// lapis 1 per IP+alias (menebak password satu link), lapis 2 per IP (menyebar ke banyak link).
//
// Catatan: c.ClientIP() hanya akurat kalau TRUSTED_PROXIES diisi saat Gateway di belakang
// reverse proxy — tanpa itu semua user terlihat dengan IP proxy dan saling berbagi kuota.
func VerifyPasswordRules(perAlias, perIP int, window time.Duration) RulesFunc {
	return func(c *gin.Context) []Rule {
		ip := c.ClientIP()
		alias := c.Param("alias")
		if len(alias) > maxAliasKeyLen {
			alias = alias[:maxAliasKeyLen]
		}
		return []Rule{
			{Key: "verify:ip:" + ip + ":alias:" + alias, Limit: perAlias, Window: window},
			{Key: "verify:ip:" + ip, Limit: perIP, Window: window},
		}
	}
}

// ForgotPasswordRules membatasi permintaan email reset per IP (mencegah email bombing dan enumerasi lewat timing).
func ForgotPasswordRules(perIP int, window time.Duration) RulesFunc {
	return func(c *gin.Context) []Rule {
		return []Rule{{Key: "forgot:ip:" + c.ClientIP(), Limit: perIP, Window: window}}
	}
}

// ResetPasswordRules membatasi percobaan reset per IP. Token 256 bit tidak bisa ditebak; ini menahan banjir request.
func ResetPasswordRules(perIP int, window time.Duration) RulesFunc {
	return func(c *gin.Context) []Rule {
		return []Rule{{Key: "reset:ip:" + c.ClientIP(), Limit: perIP, Window: window}}
	}
}

// AuthRules membatasi endpoint auth publik (register, login, refresh, OAuth) per IP.
// Terpisah dari forgot/reset yang punya kuota sendiri.
func AuthRules(perIP int, window time.Duration) RulesFunc {
	return func(c *gin.Context) []Rule {
		return []Rule{{Key: "auth:ip:" + c.ClientIP(), Limit: perIP, Window: window}}
	}
}

// PublicRules membatasi endpoint publik tanpa auth (redirect, WebSocket realtime) per IP.
func PublicRules(perIP int, window time.Duration) RulesFunc {
	return func(c *gin.Context) []Rule {
		return []Rule{{Key: "public:ip:" + c.ClientIP(), Limit: perIP, Window: window}}
	}
}

// UserRules membatasi request terautentikasi per user. Harus dipasang SETELAH middleware.RequireAuth;
// JWT dan API key milik user yang sama berbagi kuota, jadi membuat banyak API key tidak melipatgandakan batas.
func UserRules(perUser int, window time.Duration) RulesFunc {
	return func(c *gin.Context) []Rule {
		return []Rule{{Key: "user:" + middleware.UserID(c), Limit: perUser, Window: window}}
	}
}

// RefreshRules membatasi /auth/refresh per IP di bucket sendiri. Landing page memanggil endpoint ini untuk
// setiap pengunjung anonim (mengecek sesi), jadi batasnya jauh lebih longgar daripada login/register.
func RefreshRules(perIP int, window time.Duration) RulesFunc {
	return func(c *gin.Context) []Rule {
		return []Rule{{Key: "refresh:ip:" + c.ClientIP(), Limit: perIP, Window: window}}
	}
}
