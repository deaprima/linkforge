package ratelimit

import (
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
