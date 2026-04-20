package gateway

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type ipRateLimiter struct {
	mu sync.Mutex
	m  map[string]*rate.Limiter

	rate  rate.Limit
	burst int
	ttl   time.Duration
	last  map[string]time.Time
}

func newIPRateLimiter(r rate.Limit, burst int, ttl time.Duration) *ipRateLimiter {
	if burst <= 0 {
		burst = 2
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	return &ipRateLimiter{
		m:     map[string]*rate.Limiter{},
		last:  map[string]time.Time{},
		rate:  r,
		burst: burst,
		ttl:   ttl,
	}
}

func (l *ipRateLimiter) get(ip string) *rate.Limiter {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	// tiny opportunistic cleanup
	for k, ts := range l.last {
		if now.Sub(ts) > l.ttl {
			delete(l.last, k)
			delete(l.m, k)
		}
	}

	if lim, ok := l.m[ip]; ok {
		l.last[ip] = now
		return lim
	}
	lim := rate.NewLimiter(l.rate, l.burst)
	l.m[ip] = lim
	l.last[ip] = now
	return lim
}

func clientIP(r *http.Request) string {
	remote := strings.TrimSpace(r.RemoteAddr)

	// Derive remote host (best-effort).
	host := remote
	if h, _, err := net.SplitHostPort(remote); err == nil && h != "" {
		host = h
	}

	// Trust X-Forwarded-For ONLY when we are behind a controlled reverse proxy.
	// MVP rule: treat loopback as "trusted proxy" (typical local dev / compose setups).
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			parts := strings.Split(xff, ",")
			if len(parts) > 0 {
				return strings.TrimSpace(parts[0])
			}
		}
	}

	if host != "" {
		return host
	}
	return remote
}

func rateLimit(l *ipRateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if ip == "" {
			ip = "unknown"
		}
		if !l.get(ip).Allow() {
			writeError(w, http.StatusTooManyRequests, "RATE_LIMITED", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}
