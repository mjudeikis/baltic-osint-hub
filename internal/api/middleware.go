package api

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// The API is public, unauthenticated and backed by queries that group the
// whole recent incident table per request. That is fine for a dashboard and
// a few third-party pages; it is not fine for one client in a loop. These
// middlewares bound what a single request and a single client can cost.

// WithTimeout cancels the request context after d. Handlers pass it to every
// query, so a slow query is abandoned server-side rather than holding a
// connection for as long as the client cares to wait.
func WithTimeout(d time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), d)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RateLimiter is a per-client token bucket.
type RateLimiter struct {
	rps   rate.Limit
	burst int
	mu    sync.Mutex
	seen  map[string]*client
}

type client struct {
	lim  *rate.Limiter
	last time.Time
}

// NewRateLimiter allows rps sustained requests per client with the given
// burst. Idle clients are evicted every few minutes so the map does not grow
// with every address that ever visited.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	l := &RateLimiter{rps: rate.Limit(rps), burst: burst, seen: map[string]*client{}}
	go l.evictLoop()
	return l
}

// evictInterval is how long a client must be idle before it is forgotten;
// its bucket is full again by then anyway.
const evictInterval = 3 * time.Minute

func (l *RateLimiter) evictLoop() {
	for range time.Tick(evictInterval) {
		l.evict(time.Now().Add(-evictInterval))
	}
}

func (l *RateLimiter) evict(before time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for k, c := range l.seen {
		if c.last.Before(before) {
			delete(l.seen, k)
		}
	}
}

func (l *RateLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	c, ok := l.seen[key]
	if !ok {
		c = &client{lim: rate.NewLimiter(l.rps, l.burst)}
		l.seen[key] = c
	}
	c.last = time.Now()
	return c.lim.Allow()
}

// Wrap applies the limit to next, answering 429 when a client is over it.
// The 429 carries the CORS header so a browser page can tell it was
// throttled rather than seeing a generic failure.
func (l *RateLimiter) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.allow(ClientIP(r)) {
			w.Header().Set("Retry-After", "1")
			httpError(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP identifies the caller. The server sits behind Cloudflare, so the
// socket peer is always an edge node and the real client is in the headers.
// CF-Connecting-IP is set by Cloudflare itself and is preferred; the first
// entry of X-Forwarded-For is the conventional fallback. Both are only
// trustworthy because the origin is not reachable except through the edge;
// a deployment that exposes the origin directly would need to stop reading
// them, since a client can then set them freely.
func ClientIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); ip != "" {
		return ip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first, _, ok := strings.Cut(xff, ","); ok {
			return strings.TrimSpace(first)
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
