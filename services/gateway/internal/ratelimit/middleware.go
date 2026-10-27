package ratelimit

import (
	"log"
	"math"
	"net/http"
	"strconv"

	"github.com/deaprima/linkforge/services/gateway/internal/response"
	"github.com/gin-gonic/gin"
)

// RulesFunc menentukan rule untuk sebuah request (mis. berdasarkan IP dan path param).
type RulesFunc func(c *gin.Context) []Rule

type Options struct {
	// FailOpen = izinkan request saat limiter error (Redis down). Default false (fail-closed):
	// dipakai untuk endpoint sensitif di mana brute-force tidak boleh lolos.
	FailOpen bool
	// Message untuk respons 429.
	Message string
}

// Middleware menerapkan rate limit dan selalu menulis header X-RateLimit-* (lihat API-CONTRACT.md).
func Middleware(limiter Limiter, rules RulesFunc, opts Options) gin.HandlerFunc {
	if opts.Message == "" {
		opts.Message = "Too many requests. Please slow down."
	}
	return func(c *gin.Context) {
		res, err := limiter.Allow(c.Request.Context(), rules(c)...)
		if err != nil {
			log.Printf("[gateway] rate limiter unavailable on %s %s: %v", c.Request.Method, c.FullPath(), err)
			if opts.FailOpen {
				c.Next()
				return
			}
			response.Error(c, http.StatusServiceUnavailable, response.CodeServiceUnavailable, "Service is temporarily unavailable, please try again later.")
			return
		}

		h := c.Writer.Header()
		h.Set("X-RateLimit-Limit", strconv.Itoa(res.Limit))
		h.Set("X-RateLimit-Remaining", strconv.Itoa(res.Remaining))
		h.Set("X-RateLimit-Reset", strconv.FormatInt(res.Reset.Unix(), 10))

		if !res.Allowed {
			h.Set("Retry-After", strconv.Itoa(int(math.Ceil(res.RetryAfter.Seconds()))))
			response.Error(c, http.StatusTooManyRequests, response.CodeRateLimitExceeded, opts.Message)
			return
		}
		c.Next()
	}
}
