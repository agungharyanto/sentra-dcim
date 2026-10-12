// Package handler exposes system endpoints under /api/v1.
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/system/service"
)

// Handler serves /api/v1/health, /api/v1/ready and /api/v1/version.
type Handler struct{ svc *service.Service }

// New creates a Handler.
func New(svc *service.Service) *Handler { return &Handler{svc: svc} }

// Routes registers the system endpoints on r (mounted at /api/v1).
func (h *Handler) Routes(r chi.Router) {
	r.Get("/health", h.health)
	r.Get("/ready", h.ready)
	r.Get("/version", h.version)
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

func (h *Handler) version(w http.ResponseWriter, _ *http.Request) {
	httpserver.JSON(w, http.StatusOK, h.svc.Version())
}
