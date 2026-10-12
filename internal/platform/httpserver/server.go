// Package httpserver builds the HTTP router, middleware chain and server.
package httpserver

import (
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// MaxBodyBytes is the default request body limit for API requests.
const MaxBodyBytes = 1 << 20

// NewRouter returns a router with the standard middleware chain. mount registers /api/v1 routes.
// X-Real-IP is honoured only from peers inside trustedProxies.
func NewRouter(log *slog.Logger, rl *RateLimiter, trustedProxies []netip.Prefix, mount func(r chi.Router)) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(realIP(trustedProxies))
	r.Use(requestIDHeader)
	r.Use(accessLog(log))
	r.Use(recoverer(log))
	r.Use(securityHeaders)
	r.Use(maxBody(MaxBodyBytes))
	r.Use(middleware.Timeout(30 * time.Second))
	if rl != nil {
		r.Use(rl.Middleware)
	}

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		Error(w, r, http.StatusNotFound, "not_found", "resource not found")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	})

	r.Route("/api/v1", mount)
	return r
}

// NewServer returns an http.Server with conservative timeouts.
func NewServer(addr string, h http.Handler, log *slog.Logger) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      35 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 16,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
}
