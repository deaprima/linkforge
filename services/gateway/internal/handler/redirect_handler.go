package handler

import (
	"net/http"

	"github.com/deaprima/linkforge/services/gateway/internal/response"
	pb "github.com/deaprima/linkforge/services/shared/proto/link"
	"github.com/gin-gonic/gin"
)

// RedirectHandler melayani endpoint publik /:alias (tanpa auth).
type RedirectHandler struct {
	links LinkService
}

func NewRedirectHandler(links LinkService) *RedirectHandler {
	return &RedirectHandler{links: links}
}

type verifyPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

// GET /:alias
func (h *RedirectHandler) Resolve(c *gin.Context) {
	res, err := h.links.ResolveAlias(rpcContext(c), &pb.ResolveAliasRequest{Alias: c.Param("alias")})
	if err != nil {
		linkError(c, err)
		return
	}

	switch {
	case res.GetIsExpired():
		response.Error(c, http.StatusGone, response.CodeLinkExpired, "This link has expired.")
	case res.GetHasPassword():
		// Bukan error state sebenarnya: FE menampilkan form password lalu POST /:alias/verify
		response.Error(c, http.StatusOK, response.CodePasswordRequired, "This link is password protected.")
	default:
		// 302 (bukan 301) supaya browser tidak cache redirect dan setiap klik tetap tercatat
		c.Redirect(http.StatusFound, res.GetOriginalUrl())
	}
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
