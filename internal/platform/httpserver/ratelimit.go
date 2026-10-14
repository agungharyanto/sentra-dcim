package httpserver

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter is an in-memory per-client-IP token bucket limiter.
// It will be replaced by a Redis-backed limiter when the app runs multiple replicas.
type RateLimiter struct {
	rps     rate.Limit
	burst   int
	ttl     time.Duration
	mu      sync.Mutex
	clients map[string]*client
	skip    map[string]bool // exact paths never limited (see Skip)
}

type client struct {
	lim  *rate.Limiter
	seen time.Time
}

// NewRateLimiter creates a limiter allowing rps requests/second with the given burst per client IP.
func NewRateLimiter(rps float64, burst int) *RateLimiter {
	return &RateLimiter{rps: rate.Limit(rps), burst: burst, ttl: 10 * time.Minute, clients: map[string]*client{}}
}

// Skip exempts exact paths from the limit. Used for the Allowed IP decision endpoint that Caddy's forward_auth calls
// for every public request (static assets included): counting it would spend each client's budget twice and turn
// page loads into 429s. The request it guards is still limited when it reaches the API.
func (rl *RateLimiter) Skip(paths ...string) *RateLimiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if rl.skip == nil {
		rl.skip = map[string]bool{}
	}
	for _, p := range paths {
		rl.skip[p] = true
	}
	return rl
}

// Allow reports whether a request from key may proceed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	c, ok := rl.clients[key]
	if !ok {
		c = &client{lim: rate.NewLimiter(rl.rps, rl.burst)}
		rl.clients[key] = c
	}
	c.seen = time.Now()
	return c.lim.Allow()
}

// Janitor evicts idle clients until ctx is done.
func (rl *RateLimiter) Janitor(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			rl.mu.Lock()
			for k, c := range rl.clients {
				if now.Sub(c.seen) > rl.ttl {
					delete(rl.clients, k)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// Middleware rejects requests over the limit with 429.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rl.mu.Lock()
		skip := rl.skip[r.URL.Path]
		rl.mu.Unlock()
		if skip {
			next.ServeHTTP(w, r)
			return
		}
		ip, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			ip = r.RemoteAddr
		}
		if !rl.Allow(ip) {
			w.Header().Set("Retry-After", strconv.Itoa(1))
			Error(w, r, http.StatusTooManyRequests, "rate_limited", "too many requests")
			return
		}
		next.ServeHTTP(w, r)
	})
}
