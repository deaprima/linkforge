package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/deaprima/linkforge/services/gateway/internal/middleware"
	"github.com/deaprima/linkforge/services/gateway/internal/response"
	pb "github.com/deaprima/linkforge/services/shared/proto/link"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultPage  = 1
	defaultLimit = 20
	maxLimit     = 100
)

// LinkService adalah subset LinkClient yang dibutuhkan LinkHandler & RedirectHandler.
type LinkService interface {
	CreateLink(ctx context.Context, req *pb.CreateLinkRequest) (*pb.CreateLinkResponse, error)
	GetLink(ctx context.Context, req *pb.GetLinkRequest) (*pb.GetLinkResponse, error)
	ListLinks(ctx context.Context, req *pb.ListLinksRequest) (*pb.ListLinksResponse, error)
	UpdateLink(ctx context.Context, req *pb.UpdateLinkRequest) (*pb.UpdateLinkResponse, error)
	DeleteLink(ctx context.Context, req *pb.DeleteLinkRequest) (*pb.DeleteLinkResponse, error)
	ResolveAlias(ctx context.Context, req *pb.ResolveAliasRequest) (*pb.ResolveAliasResponse, error)
	VerifyLinkPassword(ctx context.Context, req *pb.VerifyLinkPasswordRequest) (*pb.VerifyLinkPasswordResponse, error)
	GetUserLinkIDs(ctx context.Context, req *pb.GetUserLinkIDsRequest) (*pb.GetUserLinkIDsResponse, error)
}

type LinkHandler struct {
	links   LinkService
	baseURL string
}

func NewLinkHandler(links LinkService, baseURL string) *LinkHandler {
	return &LinkHandler{links: links, baseURL: baseURL}
}

type linkDTO struct {
	ID          string     `json:"id"`
	OriginalURL string     `json:"original_url"`
	Alias       string     `json:"alias"`
	ShortURL    string     `json:"short_url"`
	HasPassword bool       `json:"has_password"`
	ClickCount  int64      `json:"click_count"`
	ExpiredAt   *time.Time `json:"expired_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// toLinkDTO — short_url dirakit di Gateway karena hanya Gateway yang tahu base URL publik.
func (h *LinkHandler) toLinkDTO(l *pb.Link) linkDTO {
	return linkDTO{
		ID:          l.GetId(),
		OriginalURL: l.GetOriginalUrl(),
		Alias:       l.GetAlias(),
		ShortURL:    h.baseURL + "/" + l.GetAlias(),
		HasPassword: l.GetHasPassword(),
		ClickCount:  l.GetClickCount(),
		ExpiredAt:   timePtr(l.GetExpiredAt()),
		CreatedAt:   timeVal(l.GetCreatedAt()),
		UpdatedAt:   timeVal(l.GetUpdatedAt()),
	}
}

var linkOverrides = response.Overrides{
	codes.NotFound:           {Status: http.StatusNotFound, Code: response.CodeLinkNotFound},
	codes.AlreadyExists:      {Status: http.StatusConflict, Code: response.CodeAliasTaken},
	codes.FailedPrecondition: {Status: http.StatusGone, Code: response.CodeLinkExpired},
}

// linkError menambahkan kasus MALICIOUS_URL (InvalidArgument dari blocked_domains) di atas linkOverrides.
func linkError(c *gin.Context, err error) {
	if st, ok := status.FromError(err); ok && st.Code() == codes.InvalidArgument &&
		strings.Contains(st.Message(), "is blocked") {
		response.Error(c, http.StatusBadRequest, response.CodeMaliciousURL, "The URL domain is blocked.")
		return
	}
	response.GRPCError(c, err, linkOverrides)
}

type createLinkRequest struct {
	OriginalURL string     `json:"original_url" binding:"required,http_url,max=2048"`
	Alias       string     `json:"alias" binding:"omitempty,max=64"`
	ExpiredAt   *time.Time `json:"expired_at"`
	Password    string     `json:"password" binding:"omitempty,min=4,max=72"`
}

// PUT = replace: expired_at yang tidak dikirim / null berarti hapus expiry.
// original_url & alias kosong berarti tidak diubah.
type updateLinkRequest struct {
	OriginalURL string     `json:"original_url" binding:"omitempty,http_url,max=2048"`
	Alias       string     `json:"alias" binding:"omitempty,max=64"`
	ExpiredAt   *time.Time `json:"expired_at"`
}

func expiryInPast(t *time.Time) bool {
	return t != nil && !t.After(time.Now())
}

// POST /api/v1/links
func (h *LinkHandler) Create(c *gin.Context) {
	var req createLinkRequest
	if !bindJSON(c, &req, false) {
		return
	}
	if expiryInPast(req.ExpiredAt) {
		response.Error(c, http.StatusBadRequest, response.CodeValidation, "expired_at must be in the future")
		return
	}

	res, err := h.links.CreateLink(rpcContext(c), &pb.CreateLinkRequest{
		UserId:      middleware.UserID(c),
		OriginalUrl: req.OriginalURL,
		Alias:       req.Alias,
		ExpiredAt:   toTimestamp(req.ExpiredAt),
		Password:    req.Password,
	})
	if err != nil {
		linkError(c, err)
		return
	}
	response.OK(c, http.StatusCreated, h.toLinkDTO(res.GetLink()))
}

// GET /api/v1/links?page=&limit=
func (h *LinkHandler) List(c *gin.Context) {
	page, ok := queryInt(c, "page", defaultPage)
	if !ok {
		return
	}
	limit, ok := queryInt(c, "limit", defaultLimit)
	if !ok {
		return
	}
	limit = min(limit, maxLimit)

	res, err := h.links.ListLinks(rpcContext(c), &pb.ListLinksRequest{
		UserId: middleware.UserID(c),
		Page:   int32(page),
		Limit:  int32(limit),
	})
	if err != nil {
		linkError(c, err)
		return
	}

	links := make([]linkDTO, 0, len(res.GetLinks()))
	for _, l := range res.GetLinks() {
		links = append(links, h.toLinkDTO(l))
	}
	response.OKWithMeta(c, links, response.Meta{Page: page, Limit: limit, Total: int(res.GetTotal())})
}

// GET /api/v1/links/:id
func (h *LinkHandler) Get(c *gin.Context) {
	res, err := h.links.GetLink(rpcContext(c), &pb.GetLinkRequest{
		LinkId: c.Param("id"),
		UserId: middleware.UserID(c),
	})
	if err != nil {
		linkError(c, err)
		return
	}
	response.OK(c, http.StatusOK, h.toLinkDTO(res.GetLink()))
}

// PUT /api/v1/links/:id
func (h *LinkHandler) Update(c *gin.Context) {
	var req updateLinkRequest
	if !bindJSON(c, &req, false) {
		return
	}
	if expiryInPast(req.ExpiredAt) {
		response.Error(c, http.StatusBadRequest, response.CodeValidation, "expired_at must be in the future")
		return
	}

	res, err := h.links.UpdateLink(rpcContext(c), &pb.UpdateLinkRequest{
		LinkId:      c.Param("id"),
		UserId:      middleware.UserID(c),
		OriginalUrl: req.OriginalURL,
		Alias:       req.Alias,
		ExpiredAt:   toTimestamp(req.ExpiredAt),
	})
	if err != nil {
		linkError(c, err)
		return
	}
	response.OK(c, http.StatusOK, h.toLinkDTO(res.GetLink()))
}

// DELETE /api/v1/links/:id
func (h *LinkHandler) Delete(c *gin.Context) {
	_, err := h.links.DeleteLink(rpcContext(c), &pb.DeleteLinkRequest{
		LinkId: c.Param("id"),
		UserId: middleware.UserID(c),
	})
	if err != nil {
		linkError(c, err)
		return
	}
	response.NoContent(c)
}

func queryInt(c *gin.Context, key string, def int) (int, bool) {
	raw := c.Query(key)
	if raw == "" {
		return def, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		response.Error(c, http.StatusBadRequest, response.CodeValidation, key+" must be a positive integer")
		return 0, false
	}
	return n, true
}
