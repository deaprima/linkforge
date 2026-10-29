// Package ratelimit adalah rate limiter fixed-window berbasis Redis untuk Gateway.
// Dipakai untuk brute-force protection POST /:alias/verify, rate limit umum per IP / user,
// dan endpoint sensitif lain (forgot/reset password).
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Rule: maksimal Limit request per Window untuk Key.
type Rule struct {
	Key    string
	Limit  int
	Window time.Duration
}

// Result menggambarkan rule yang paling dekat ke batas (dipakai untuk header X-RateLimit-*).
type Result struct {
	Allowed    bool
	Limit      int
	Remaining  int
	Reset      time.Time     // kapan window rule tsb berakhir
	RetryAfter time.Duration // > 0 hanya kalau ditolak
}

type Limiter interface {
	// Allow mencatat satu request untuk SEMUA rule secara atomik; ditolak kalau salah satu terlampaui.
	Allow(ctx context.Context, rules ...Rule) (Result, error)
}

// Per key: INCR, set expiry hanya pada request pertama di window (fixed window — request
// berikutnya tidak memperpanjang window), lalu kembalikan {count, sisa ttl ms}.
var fixedWindowScript = redis.NewScript(`
local out = {}
for i, key in ipairs(KEYS) do
	local count = redis.call("INCR", key)
	if count == 1 then
		redis.call("PEXPIRE", key, ARGV[i])
	end
	local ttl = redis.call("PTTL", key)
	if ttl < 0 then
		redis.call("PEXPIRE", key, ARGV[i])
		ttl = tonumber(ARGV[i])
	end
	out[#out + 1] = count
	out[#out + 1] = ttl
end
return out
`)

type RedisLimiter struct {
	rdb    redis.Scripter
	prefix string
	now    func() time.Time
}

func NewRedisLimiter(rdb redis.Scripter, prefix string) *RedisLimiter {
	return &RedisLimiter{rdb: rdb, prefix: prefix, now: time.Now}
}

func (l *RedisLimiter) Allow(ctx context.Context, rules ...Rule) (Result, error) {
	if len(rules) == 0 {
		return Result{Allowed: true}, nil
	}
	keys := make([]string, len(rules))
	windows := make([]any, len(rules))
	for i, r := range rules {
		keys[i] = l.prefix + r.Key
		windows[i] = r.Window.Milliseconds()
	}

	raw, err := fixedWindowScript.Run(ctx, l.rdb, keys, windows...).Int64Slice()
	if err != nil {
		return Result{}, fmt.Errorf("rate limiter: %w", err)
	}
	if len(raw) != 2*len(rules) {
		return Result{}, fmt.Errorf("rate limiter: unexpected script result length %d", len(raw))
	}

	now := l.now()
	var tightest, blocked *Result
	for i, r := range rules {
		count, ttl := raw[2*i], time.Duration(raw[2*i+1])*time.Millisecond
		res := Result{
			Allowed:   count <= int64(r.Limit),
			Limit:     r.Limit,
			Remaining: max(r.Limit-int(count), 0),
			Reset:     now.Add(ttl),
		}
		if !res.Allowed {
			res.RetryAfter = ttl
			// Kalau beberapa rule terlampaui, client harus menunggu yang paling lama
			if blocked == nil || res.RetryAfter > blocked.RetryAfter {
				blocked = &res
			}
		}
		if tightest == nil || res.Remaining < tightest.Remaining {
			tightest = &res
		}
	}
	if blocked != nil {
		return *blocked, nil
	}
	return *tightest, nil
}
