// Package ratelimit implements shared GCRA rate limiting on Redis (§13.3).
package ratelimit

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/go-redis/redis_rate/v10"
	"github.com/redis/go-redis/v9"
)

type Limit struct {
	Rate   int
	Period time.Duration
}

func PerMinute(n int) Limit { return Limit{Rate: n, Period: time.Minute} }
func PerHour(n int) Limit   { return Limit{Rate: n, Period: time.Hour} }
func PerDay(n int) Limit    { return Limit{Rate: n, Period: 24 * time.Hour} }

type Result struct {
	Allowed    bool
	RetryAfter time.Duration
}

type Limiter struct {
	l      *redis_rate.Limiter
	prefix string
}

func New(rdb redis.UniversalClient) *Limiter {
	return &Limiter{l: redis_rate.NewLimiter(rdb), prefix: "rl:"}
}

// Allow consumes one token for key against every limit; it is denied as soon
// as one of them is exhausted. Errors (Redis down) are returned to the caller,
// which decides between fail-open and fail-closed.
func (r *Limiter) Allow(ctx context.Context, key string, limits ...Limit) (Result, error) {
	res := Result{Allowed: true}
	for _, lim := range limits {
		rl := redis_rate.Limit{Rate: lim.Rate, Burst: lim.Rate, Period: lim.Period}
		out, err := r.l.Allow(ctx, r.prefix+key+":"+lim.Period.String(), rl)
		if err != nil {
			return Result{Allowed: true}, err
		}
		if out.Allowed == 0 {
			res.Allowed = false
			if out.RetryAfter > res.RetryAfter {
				res.RetryAfter = out.RetryAfter
			}
		}
	}
	return res, nil
}

// Reset clears the counters of key (e.g. after a successful login).
func (r *Limiter) Reset(ctx context.Context, key string, limits ...Limit) error {
	for _, lim := range limits {
		if err := r.l.Reset(ctx, r.prefix+key+":"+lim.Period.String()); err != nil {
			return err
		}
	}
	return nil
}

// IPKey normalises an IP for rate limiting: IPv4 as is, IPv6 grouped by /64.
func IPKey(ip string) string {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return "unknown"
	}
	if v4 := parsed.To4(); v4 != nil {
		return v4.String()
	}
	return parsed.Mask(net.CIDRMask(64, 128)).String() + "/64"
}
