package middleware

import (
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"vtv.vn/backend/pkg/xhttp"
)

// IPRateLimiter throttles requests by client IP using a token-bucket per IP.
// Idle visitors are evicted on a sweep to bound memory.
//
// Usage: limiter := NewIPRateLimiter(rate.Every(time.Second), 10)
// e.Use(limiter.Middleware())
type IPRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rate     rate.Limit
	burst    int
	ttl      time.Duration
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewIPRateLimiter builds an IP rate limiter that produces `r` tokens per second
// with bucket size `burst`. Entries older than 10 minutes are pruned.
func NewIPRateLimiter(r rate.Limit, burst int) *IPRateLimiter {
	l := &IPRateLimiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    burst,
		ttl:      10 * time.Minute,
	}
	go l.sweep()
	return l
}

func (l *IPRateLimiter) get(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()
	v, ok := l.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(l.rate, l.burst)}
		l.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func (l *IPRateLimiter) sweep() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for range t.C {
		l.mu.Lock()
		cutoff := time.Now().Add(-l.ttl)
		for ip, v := range l.visitors {
			if v.lastSeen.Before(cutoff) {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

// Middleware enforces the rate limit per client IP.
func (l *IPRateLimiter) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := c.RealIP()
			if !l.get(ip).Allow() {
				return xhttp.AppErrorResponse(c, xhttp.TooManyRequestsErrorf("quá nhiều yêu cầu, vui lòng thử lại sau"))
			}
			return next(c)
		}
	}
}
