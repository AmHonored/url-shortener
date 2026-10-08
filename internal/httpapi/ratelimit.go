package httpapi

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// RateLimiter tracks request counts per IP in a fixed one-minute window.
type RateLimiter struct {
	mu      sync.Mutex
	counts  map[string]int
	limit   int
	resetAt time.Time
}

// NewRateLimiter returns a limiter that allows limit requests per IP per minute.
func NewRateLimiter(limit int) *RateLimiter {
	return &RateLimiter{
		counts:  make(map[string]int),
		limit:   limit,
		resetAt: time.Now().Add(time.Minute),
	}
}

// Allow returns true if the IP has not exceeded the limit.
func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	if now.After(rl.resetAt) {
		rl.counts = make(map[string]int)
		rl.resetAt = now.Add(time.Minute)
	}

	rl.counts[ip]++
	return rl.counts[ip] <= rl.limit
}

func clientIP(r *http.Request) string {
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
