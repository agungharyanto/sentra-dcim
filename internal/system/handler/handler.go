// Package handler exposes system endpoints under /api/v1.
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
	"github.com/agungharyanto/sentra-dcim/internal/system/service"
)

// Handler serves /api/v1/health, /api/v1/ready and /api/v1/version.
type Handler struct {
	svc *service.Service
}

// New creates a Handler. /version returns only {"version"} unless the caller is signed in with the permission
// system.version.read (Admin, D-054/D-059) — on both listeners; mount it behind an Identify middleware.
func New(svc *service.Service) *Handler { return &Handler{svc: svc} }

// Routes registers all system endpoints on r (mounted at /api/v1) without caller identification (tests).
func (h *Handler) Routes(r chi.Router) {
	h.PublicRoutes(r, func(next http.Handler) http.Handler { return next })
	h.OpsRoutes(r)
}

// PublicRoutes registers the endpoints that never require a session: /health and /version. identify optionally
// resolves the session so /version can return details to callers allowed to see them.
func (h *Handler) PublicRoutes(r chi.Router, identify func(http.Handler) http.Handler) {
	r.Get("/health", h.health)
	r.With(identify).Get("/version", h.version)
}

// OpsRoutes registers /ready (dependency status): unauthenticated on the internal listener only.
func (h *Handler) OpsRoutes(r chi.Router) {
	r.Get("/ready", h.ready)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	httpserver.JSON(w, http.StatusOK, map[string]string{"status": service.StatusOK})
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	rep := h.svc.Readiness(r.Context())
	code := http.StatusOK
	if rep.Status != service.StatusReady {
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, code, rep)
}

func (h *Handler) version(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if p, ok := principal.From(r.Context()); ok && p.Can(rbac.SystemVersionRead) {
		httpserver.JSON(w, http.StatusOK, h.svc.Version())
		return
	}
	httpserver.JSON(w, http.StatusOK, h.svc.PublicVersion())
}
