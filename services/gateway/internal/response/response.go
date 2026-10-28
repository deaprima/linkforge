// Package response membungkus semua response publik Gateway ke format envelope standar
// (lihat docs/API-CONTRACT.md bagian "Format Response Standar").
package response

import (
	"errors"
	"log"
	"net/http"

	"github.com/deaprima/linkforge/services/shared/breaker"
	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Error code publik — sinkron dengan "Daftar Error Code" di API-CONTRACT.md
const (
	CodeValidation         = "VALIDATION_ERROR"
	CodeMaliciousURL       = "MALICIOUS_URL"
	CodeInvalidResetToken  = "INVALID_RESET_TOKEN"
	CodePasswordRequired   = "PASSWORD_REQUIRED"
	CodeUnauthorized       = "UNAUTHORIZED"
	CodeForbidden          = "FORBIDDEN"
	CodeNotFound           = "NOT_FOUND"
	CodeLinkNotFound       = "LINK_NOT_FOUND"
	CodeLinkExpired        = "LINK_EXPIRED"
	CodeEmailExists        = "EMAIL_ALREADY_EXISTS"
	CodeAliasTaken         = "ALIAS_ALREADY_TAKEN"
	CodeConflict           = "CONFLICT"
	CodeRateLimitExceeded  = "RATE_LIMIT_EXCEEDED"
	CodeServiceUnavailable = "SERVICE_UNAVAILABLE"
	CodeInternal           = "INTERNAL_ERROR"
)

type Meta struct {
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
}

type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type successEnvelope struct {
	Success bool  `json:"success"`
	Data    any   `json:"data"`
	Meta    *Meta `json:"meta,omitempty"`
}

type errorEnvelope struct {
	Success bool      `json:"success"`
	Error   ErrorBody `json:"error"`
}

func OK(c *gin.Context, httpStatus int, data any) {
	c.JSON(httpStatus, successEnvelope{Success: true, Data: data})
}

func OKWithMeta(c *gin.Context, data any, meta Meta) {
	c.JSON(http.StatusOK, successEnvelope{Success: true, Data: data, Meta: &meta})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// Error menulis error envelope dan menghentikan handler chain (aman dipakai di middleware).
func Error(c *gin.Context, httpStatus int, code, message string) {
	c.AbortWithStatusJSON(httpStatus, errorEnvelope{
		Success: false,
		Error:   ErrorBody{Code: code, Message: message},
	})
}

// Mapping menentukan status HTTP + error code publik untuk sebuah gRPC code.
type Mapping struct {
	Status int
	Code   string
}

// Overrides dipakai handler untuk mengganti mapping default sesuai konteks
// (mis. AlreadyExists = EMAIL_ALREADY_EXISTS di register, ALIAS_ALREADY_TAKEN di link).
type Overrides map[codes.Code]Mapping

var defaultMappings = map[codes.Code]Mapping{
	codes.InvalidArgument:    {http.StatusBadRequest, CodeValidation},
	codes.FailedPrecondition: {http.StatusBadRequest, CodeValidation},
	codes.Unauthenticated:    {http.StatusUnauthorized, CodeUnauthorized},
	codes.PermissionDenied:   {http.StatusForbidden, CodeForbidden},
	codes.NotFound:           {http.StatusNotFound, CodeNotFound},
	codes.AlreadyExists:      {http.StatusConflict, CodeConflict},
	codes.ResourceExhausted:  {http.StatusTooManyRequests, CodeRateLimitExceeded},
	codes.Unavailable:        {http.StatusServiceUnavailable, CodeServiceUnavailable},
	codes.DeadlineExceeded:   {http.StatusServiceUnavailable, CodeServiceUnavailable},
}

// GRPCError menerjemahkan error dari gRPC client (termasuk circuit breaker open) ke error envelope.
// Pesan error 4xx diteruskan apa adanya; pesan 5xx disembunyikan supaya detail internal tidak bocor.
func GRPCError(c *gin.Context, err error, overrides Overrides) {
	if errors.Is(err, breaker.ErrOpen) {
		log.Printf("[gateway] circuit breaker open, rejecting %s %s", c.Request.Method, c.FullPath())
		Error(c, http.StatusServiceUnavailable, CodeServiceUnavailable, "Service is temporarily unavailable, please try again later.")
		return
	}

	st, ok := status.FromError(err)
	if !ok {
		log.Printf("[gateway] unexpected non-gRPC error on %s %s: %v", c.Request.Method, c.FullPath(), err)
		Error(c, http.StatusInternalServerError, CodeInternal, "An unexpected error occurred.")
		return
	}

	m, found := overrides[st.Code()]
	if !found {
		m, found = defaultMappings[st.Code()]
	}
	if !found {
		m = Mapping{http.StatusInternalServerError, CodeInternal}
	}

	switch {
	case m.Status == http.StatusServiceUnavailable:
		log.Printf("[gateway] dependency unavailable on %s %s: %v", c.Request.Method, c.FullPath(), err)
		Error(c, m.Status, m.Code, "Service is temporarily unavailable, please try again later.")
	case m.Status >= http.StatusInternalServerError:
		log.Printf("[gateway] internal error on %s %s: %v", c.Request.Method, c.FullPath(), err)
		Error(c, m.Status, m.Code, "An unexpected error occurred.")
	default:
		Error(c, m.Status, m.Code, st.Message())
	}
}
