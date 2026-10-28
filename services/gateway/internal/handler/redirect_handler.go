package handler

import (
	"errors"
	"net/http"

	"github.com/deaprima/linkforge/services/gateway/internal/response"
	"github.com/deaprima/linkforge/services/shared/breaker"
	pb "github.com/deaprima/linkforge/services/shared/proto/link"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RedirectHandler melayani endpoint publik /:alias (tanpa auth).
type RedirectHandler struct {
	links       LinkService
	frontendURL string
}

func NewRedirectHandler(links LinkService, frontendURL string) *RedirectHandler {
	return &RedirectHandler{links: links, frontendURL: frontendURL}
}

type verifyPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// GET /:alias
func (h *RedirectHandler) Resolve(c *gin.Context) {
	alias := c.Param("alias")
	browser := wantsHTML(c)

	res, err := h.links.ResolveAlias(rpcContext(c), &pb.ResolveAliasRequest{Alias: alias})
	if err != nil {
		// Pengunjung manusia diarahkan ke halaman web app, bukan JSON mentah. Backend down punya
		// halaman sendiri (503) supaya tidak menyamar jadi "not found"; error lain tetap JSON.
		if browser {
			switch {
			case status.Code(err) == codes.NotFound:
				c.Redirect(http.StatusFound, webURL(h.frontendURL, WebPathNotFound, alias))
				return
			case backendUnavailable(err):
				c.Redirect(http.StatusFound, webURL(h.frontendURL, WebPathUnavailable, alias))
				return
			}
		}
		linkError(c, err)
		return
	}

	switch {
	case res.GetIsExpired():
		if browser {
			c.Redirect(http.StatusFound, webURL(h.frontendURL, WebPathExpired, alias))
			return
		}
		response.Error(c, http.StatusGone, response.CodeLinkExpired, "This link has expired.")
	case res.GetHasPassword():
		if browser {
			c.Redirect(http.StatusFound, webURL(h.frontendURL, WebPathPassword, alias))
			return
		}
		// Bukan error state sebenarnya: FE menampilkan form password lalu POST /:alias/verify
		response.Error(c, http.StatusOK, response.CodePasswordRequired, "This link is password protected.")
	default:
		// 302 (bukan 301) supaya browser tidak cache redirect dan setiap klik tetap tercatat
		c.Redirect(http.StatusFound, res.GetOriginalUrl())
	}
}

// backendUnavailable: Link Service tidak bisa dihubungi (bukan error bisnis) — sama dengan
// kondisi yang dipetakan GRPCError ke 503.
func backendUnavailable(err error) bool {
	if errors.Is(err, breaker.ErrOpen) {
		return true
	}
	code := status.Code(err)
	return code == codes.Unavailable || code == codes.DeadlineExceeded
}

// POST /:alias/verify
func (h *RedirectHandler) Verify(c *gin.Context) {
	var req verifyPasswordRequest
	if !bindJSON(c, &req, false) {
		return
	}

	res, err := h.links.VerifyLinkPassword(rpcContext(c), &pb.VerifyLinkPasswordRequest{
		Alias:    c.Param("alias"),
		Password: req.Password,
	})
	if err != nil {
		// Password salah = Unauthenticated → 401 UNAUTHORIZED (mapping default)
		linkError(c, err)
		return
	}
	if !res.GetIsValid() {
		response.Error(c, http.StatusUnauthorized, response.CodeUnauthorized, "Invalid password.")
		return
	}

	response.OK(c, http.StatusOK, gin.H{"redirect_url": res.GetOriginalUrl()})
}
