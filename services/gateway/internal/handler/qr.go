package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"github.com/deaprima/linkforge/services/gateway/internal/middleware"
	"github.com/deaprima/linkforge/services/gateway/internal/response"
	pb "github.com/deaprima/linkforge/services/shared/proto/link"
	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"
)

const (
	defaultQRSize = 256
	minQRSize     = 64
	maxQRSize     = 1024 // batas atas supaya request tidak bisa memaksa render gambar raksasa
)

// GET /api/v1/links/:id/qr?size=&format=png|svg
func (h *LinkHandler) QR(c *gin.Context) {
	size := defaultQRSize
	if raw := c.Query("size"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < minQRSize || n > maxQRSize {
			response.Error(c, http.StatusBadRequest, response.CodeValidation,
				fmt.Sprintf("size must be an integer between %d and %d", minQRSize, maxQRSize))
			return
		}
		size = n
	}
	format := c.DefaultQuery("format", "png")
	if format != "png" && format != "svg" {
		response.Error(c, http.StatusBadRequest, response.CodeValidation, "format must be png or svg")
		return
	}

	// Ownership dicek lewat Link Service (404 kalau bukan milik user)
	res, err := h.links.GetLink(rpcContext(c), &pb.GetLinkRequest{LinkId: c.Param("id"), UserId: middleware.UserID(c)})
	if err != nil {
		linkError(c, err)
		return
	}
	alias := res.GetLink().GetAlias()

	qr, err := qrcode.New(h.baseURL+"/"+alias, qrcode.Medium)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to generate QR code.")
		return
	}

	// QR hanya bergantung pada alias → aman di-cache browser (private: butuh auth)
	c.Header("Cache-Control", "private, max-age=3600")
	c.Header("Content-Disposition", fmt.Sprintf(`inline; filename="%s.%s"`, alias, format))

	if format == "svg" {
		c.Data(http.StatusOK, "image/svg+xml", renderSVG(qr.Bitmap(), size))
		return
	}
	png, err := qr.PNG(size)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternal, "Failed to generate QR code.")
		return
	}
	c.Data(http.StatusOK, "image/png", png)
}

// renderSVG menggambar bitmap QR (sudah termasuk quiet zone) sebagai SVG.
// Modul hitam yang berurutan dalam satu baris digabung jadi satu <rect> supaya file kecil.
func renderSVG(bitmap [][]bool, size int) []byte {
	n := len(bitmap)
	var b bytes.Buffer
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" shape-rendering="crispEdges">`, n, n, size, size)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/>`, n, n)
	b.WriteString(`<g fill="#000">`)
	for y, row := range bitmap {
		for x := 0; x < len(row); {
			if !row[x] {
				x++
				continue
			}
			start := x
			for x < len(row) && row[x] {
				x++
			}
			fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="1"/>`, start, y, x-start)
		}
	}
	b.WriteString(`</g></svg>`)
	return b.Bytes()
}
