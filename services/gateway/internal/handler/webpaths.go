package handler

import (
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
)

// Path halaman web app (relatif ke FRONTEND_URL). Semua di bawah /app supaya tidak bentrok
// dengan short link di root — lihat API-CONTRACT.md bagian Redirect (Publik).
const (
	WebPathDashboard = "/app/dashboard"
	WebPathLogin     = "/app/login"
	WebPathPassword  = "/app/p/" // + alias
	WebPathNotFound  = "/app/not-found"
	WebPathExpired   = "/app/expired"
)

// wantsHTML: pengunjung via browser (navigasi biasa mengirim Accept: text/html,...).
// Client API/CLI tidak mengirim text/html, jadi tetap menerima JSON.
func wantsHTML(c *gin.Context) bool {
	return strings.Contains(c.GetHeader("Accept"), "text/html")
}

func webURL(frontendURL, path, alias string) string {
	if path == WebPathPassword {
		return frontendURL + path + url.PathEscape(alias)
	}
	return frontendURL + path + "?alias=" + url.QueryEscape(alias)
}
