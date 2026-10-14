// Package handler exposes UI support endpoints under /api/v1/ui.
package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/ui/service"
)

// Handler serves /api/v1/ui/bootstrap (session required, default-deny in internal/api).
type Handler struct {
	svc *service.Service
}

// New creates a Handler.
func New(svc *service.Service) *Handler { return &Handler{svc: svc} }

// Routes registers the UI endpoints on r (mounted at /api/v1).
func (h *Handler) Routes(r chi.Router) {
	r.Get("/ui/bootstrap", h.bootstrap)
}

func (h *Handler) bootstrap(w http.ResponseWriter, r *http.Request) {
	// Locale/timezone come from server config and may change on restart; never cache across deploys.
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, http.StatusOK, h.svc.Bootstrap(r.Context()))
}
