package gateway

import (
	"fmt"
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
	return clientIPWithTrusted(r, nil)
}

func parseTrustedProxyCIDRs(cidrs []string) ([]*net.IPNet, error) {
	out := make([]*net.IPNet, 0, len(cidrs))
	for _, s := range cidrs {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		_, n, err := net.ParseCIDR(s)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy cidr %q: %w", s, err)
		}
		out = append(out, n)
	}
	return out, nil
}

func isTrustedProxyHost(host string, trusted []*net.IPNet) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	// MVP default: treat loopback as trusted proxy (local dev / compose).
	if ip.IsLoopback() {
		return true
	}
	for _, n := range trusted {
		if n != nil && n.Contains(ip) {
			return true
		}
	}
	return false
}

// clientIPWithTrusted returns the effective client IP for rate limiting.
//
// Security rule:
// - By default we trust only loopback proxies (local dev/compose).
// - Additionally, callers can provide trusted proxy CIDRs; only when RemoteAddr is within
//   those CIDRs do we honor X-Forwarded-For.
func clientIPWithTrusted(r *http.Request, trusted []*net.IPNet) string {
	remote := strings.TrimSpace(r.RemoteAddr)

	// Derive remote host (best-effort).
	host := remote
	if h, _, err := net.SplitHostPort(remote); err == nil && h != "" {
		host = h
	}

	// Trust X-Forwarded-For ONLY when we are behind a controlled reverse proxy.
	if isTrustedProxyHost(host, trusted) {
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
	return rateLimitWithTrusted(l, nil, next)
}

func rateLimitWithTrusted(l *ipRateLimiter, trusted []*net.IPNet, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIPWithTrusted(r, trusted)
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
