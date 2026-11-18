package web

import (
	"net"
	"net/http"
	"sync"
	"time"
)

// securityHeaders sets conservative browser protections. Inline scripts and
// styles are part of the templates, so they stay allowed; images come from
// Divine Pride's CDN.
func securityHeaders(next http.Handler) http.Handler {
	const csp = "default-src 'self'; img-src 'self' data: https://static.divine-pride.net; " +
		"script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; connect-src 'self'; " +
		"frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

// staticCache lets browsers keep the CSS/JS for an hour (files aren't
// fingerprinted, so not longer).
func staticCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		next.ServeHTTP(w, r)
	})
}

// rateLimiter is a per-IP token bucket: rate tokens per second, up to burst.
type rateLimiter struct {
	mu      sync.Mutex
	rate    float64
	burst   float64
	buckets map[string]*bucket
	lastGC  time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func newRateLimiter(perSecond, burst float64) *rateLimiter {
	return &rateLimiter{rate: perSecond, burst: burst, buckets: map[string]*bucket{}, lastGC: time.Now()}
}

func (l *rateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if now.Sub(l.lastGC) > 5*time.Minute { // forget idle clients
		for k, b := range l.buckets {
			if now.Sub(b.last) > 10*time.Minute {
				delete(l.buckets, k)
			}
		}
		l.lastGC = now
	}
	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: l.burst, last: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.burst, b.tokens+now.Sub(b.last).Seconds()*l.rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func clientIP(r *http.Request) string {
	if h, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return h
	}
	return r.RemoteAddr // already a bare IP after middleware.RealIP
}

// limit answers 429 when the client exceeds the limiter.
func (l *rateLimiter) limit(next http.Handler) http.Handler {
	if l == nil {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(clientIP(r), time.Now()) {
			w.Header().Set("Retry-After", "5")
			http.Error(w, tr(lang(r), "muitas requisições, tente de novo em alguns segundos"), http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// oneAtATime caps how many requests run a heavy handler at once (replay
// uploads are parsed in memory).
func oneAtATime(n int, next http.Handler) http.Handler {
	sem := make(chan struct{}, n)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			next.ServeHTTP(w, r)
		case <-r.Context().Done():
		case <-time.After(20 * time.Second):
			http.Error(w, tr(lang(r), "servidor ocupado, tente de novo"), http.StatusServiceUnavailable)
		}
	})
}
