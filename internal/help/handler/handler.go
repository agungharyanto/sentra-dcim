// Package handler exposes the Help Center under /api/v1/help (pages, images, OpenAPI). Every route needs a session
// that passed MFA (default-deny, internal/api). Pages are filtered by the caller's permissions (ISS-032, D-074):
// audience "admin" needs rbac.HelpAdmin, and a page bound to a menu that needs a permission — or any page below it —
// needs that permission too. Hidden pages are neither listed nor fetchable (404), and the OpenAPI document follows
// the admin page "api".
package handler

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/agungharyanto/sentra-dcim/internal/help/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
	uiservice "github.com/agungharyanto/sentra-dcim/internal/ui/service"
)

// Handler serves the help index, pages and the OpenAPI document.
type Handler struct {
	cat           *service.Catalog // nil = help module disabled (content failed validation at startup)
	defaultLocale string
	locales       []string
	menuPerms     map[string]rbac.Permission // menu key → permission needed to see it (UI registry)
}

// New creates a Handler. A nil catalog disables the module: every endpoint answers 503 while the rest of the API
// keeps working.
func New(cat *service.Catalog, defaultLocale string, locales []string) *Handler {
	_, menus := uiservice.Registry()
	perms := map[string]rbac.Permission{}
	for _, m := range menus {
		if m.Permission != "" {
			perms[m.Key] = rbac.Permission(m.Permission)
		}
	}
	return &Handler{cat: cat, defaultLocale: defaultLocale, locales: slices.Clone(locales), menuPerms: perms}
}

// can reports whether the caller holds perm (no principal = nothing).
func can(r *http.Request, perm rbac.Permission) bool {
	p, ok := principal.From(r.Context())
	return ok && p.Can(perm)
}

// allowed applies the rules of one page's metadata.
func (h *Handler) allowed(r *http.Request, m service.Meta) bool {
	if m.Audience == service.AudienceAdmin && !can(r, rbac.HelpAdmin) {
		return false
	}
	if perm, ok := h.menuPerms[m.Menu]; ok && !can(r, perm) {
		return false
	}
	return true
}

// visible checks a page and every ancestor ("users/roles" → "users"): a sub-page inherits its parent's restrictions.
func (h *Handler) visible(r *http.Request, loc string, m service.Meta) bool {
	if !h.allowed(r, m) {
		return false
	}
	for slug := m.Slug; strings.Contains(slug, "/"); {
		slug = slug[:strings.LastIndex(slug, "/")]
		if parent, ok := h.cat.Page(loc, slug); ok && !h.allowed(r, parent.Meta) {
			return false
		}
	}
	return true
}

// Routes registers the help endpoints on r (mounted at /api/v1).
func (h *Handler) Routes(r chi.Router) {
	r.Route("/help", func(r chi.Router) {
		r.Use(h.available)
		r.Get("/index", h.index)
		r.Get("/pages/*", h.page)
		r.Get("/openapi", h.openAPI)
		r.Get("/assets/{name}", h.asset)
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
	all, _ := h.cat.Index(loc)
	pages := make([]service.Meta, 0, len(all))
	for _, m := range all {
		if h.visible(r, loc, m) {
			pages = append(pages, m)
		}
	}
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
	if !found || !h.visible(r, loc, page.Meta) { // hidden pages are indistinguishable from missing ones
		httpserver.Error(w, r, http.StatusNotFound, "not_found", "help page not found")
		return
	}
	httpserver.JSON(w, http.StatusOK, page)
}

func (h *Handler) openAPI(w http.ResponseWriter, r *http.Request) {
	if !can(r, rbac.HelpAdmin) { // the API reference is the admin page "api"
		httpserver.Error(w, r, http.StatusNotFound, "not_found", "help page not found")
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(h.cat.OpenAPI())
}

// asset serves an embedded help image (whitelist lookup only; the name never reaches the filesystem).
func (h *Handler) asset(w http.ResponseWriter, r *http.Request) {
	a, ok := h.cat.Asset(chi.URLParam(r, "name"))
	if !ok {
		httpserver.Error(w, r, http.StatusNotFound, "not_found", "asset not found")
		return
	}
	w.Header().Set("Content-Type", a.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(a.Data)))
	_, _ = w.Write(a.Data)
}
