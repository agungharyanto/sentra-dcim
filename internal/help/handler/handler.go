// Package handler exposes the Help Center under /api/v1/help. Until Auth (v0.3) the public routes are guarded by
// Caddy basic_auth; from v0.3 pages are filtered by role via their audience (ISS-032).
package handler

import (
	"net/http"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/agungharyanto/sentra-dcim/internal/help/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
)

// Handler serves the help index, pages and the OpenAPI document.
type Handler struct {
	cat           *service.Catalog // nil = help module disabled (content failed validation at startup)
	defaultLocale string
	locales       []string
}

// New creates a Handler. A nil catalog disables the module: every endpoint answers 503 while the rest of the API
// keeps working.
func New(cat *service.Catalog, defaultLocale string, locales []string) *Handler {
	return &Handler{cat: cat, defaultLocale: defaultLocale, locales: slices.Clone(locales)}
}

// Routes registers the help endpoints on r (mounted at /api/v1).
func (h *Handler) Routes(r chi.Router) {
	r.Route("/help", func(r chi.Router) {
		r.Use(h.available)
		r.Get("/index", h.index)
		r.Get("/pages/*", h.page)
		r.Get("/openapi", h.openAPI)
	})
}

func (h *Handler) available(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.cat == nil {
			httpserver.Error(w, r, http.StatusServiceUnavailable, "help_unavailable", "help center is temporarily unavailable")
			return
		}
		// Content ships with the binary; revalidate so a deploy is picked up immediately.
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) locale(w http.ResponseWriter, r *http.Request) (string, bool) {
	loc := r.URL.Query().Get("locale")
	if loc == "" {
		return h.defaultLocale, true
	}
	if !slices.Contains(h.locales, loc) {
		httpserver.Error(w, r, http.StatusBadRequest, "invalid_locale", "unsupported locale")
		return "", false
	}
	return loc, true
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	loc, ok := h.locale(w, r)
	if !ok {
		return
	}
	pages, _ := h.cat.Index(loc)
	httpserver.JSON(w, http.StatusOK, map[string]any{"locale": loc, "pages": pages})
}

func (h *Handler) page(w http.ResponseWriter, r *http.Request) {
	loc, ok := h.locale(w, r)
	if !ok {
		return
	}
	// The slug is looked up in the in-memory whitelist only; it never reaches the filesystem.
	slug := strings.TrimSuffix(chi.URLParam(r, "*"), "/")
	page, found := h.cat.Page(loc, slug)
	if !found {
		httpserver.Error(w, r, http.StatusNotFound, "not_found", "help page not found")
		return
	}
	httpserver.JSON(w, http.StatusOK, page)
}

func (h *Handler) openAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.cat.OpenAPI())
}
