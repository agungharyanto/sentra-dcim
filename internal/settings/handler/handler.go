// Package handler exposes system settings under /api/v1/settings/{group} (rbac.SettingsManage, Admin) and the
// caller's own preferences under /api/v1/me/preferences (every signed-in user), v0.3.5.
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
	"github.com/agungharyanto/sentra-dcim/internal/settings/service"
)

// Stable error codes (translated in the UI: errors:code.<code>).
const (
	CodeValidation = "validation_failed"
	CodeNotFound   = "not_found"
	CodeInvalidReq = "invalid_request"
)

// Handler serves settings and preferences.
type Handler struct {
	svc *service.Service
	log *slog.Logger
}

// New creates a Handler.
func New(svc *service.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

// Routes registers the endpoints; require returns the permission middleware (auth handler Require).
func (h *Handler) Routes(r chi.Router, require func(rbac.Permission) func(http.Handler) http.Handler) {
	manage := require(rbac.SettingsManage)
	r.With(manage).Get("/settings/{group}", h.get)
	r.With(manage).Put("/settings/{group}", h.put)
	r.Get("/me/preferences", h.preferences)
	r.Put("/me/preferences", h.putPreferences)
}

// Item is one setting in API form.
type Item struct {
	service.Item
	Name string `json:"name"` // key without the group prefix (used in PUT bodies)
}

type groupResponse struct {
	Group string `json:"group"`
	Items []Item `json:"items"`
}

func (h *Handler) respondGroup(w http.ResponseWriter, group string, items []service.Item) {
	out := groupResponse{Group: group, Items: make([]Item, 0, len(items))}
	for _, it := range items {
		out.Items = append(out.Items, Item{Item: it, Name: it.Name()})
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, http.StatusOK, out)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	group := chi.URLParam(r, "group")
	items, err := h.svc.Group(r.Context(), group)
	if err != nil {
		h.fail(w, r, "get settings", err)
		return
	}
	h.respondGroup(w, group, items)
}

type valuesBody struct {
	Values map[string]json.RawMessage `json:"values"`
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		httpserver.Error(w, r, http.StatusBadRequest, CodeInvalidReq, "invalid JSON body")
		return false
	}
	return true
}

func (h *Handler) put(w http.ResponseWriter, r *http.Request) {
	group := chi.URLParam(r, "group")
	var in valuesBody
	if !decode(w, r, &in) {
		return
	}
	items, err := h.svc.Update(r.Context(), group, in.Values, audit.MetaFromRequest(r))
	if err != nil {
		h.fail(w, r, "update settings", err)
		return
	}
	h.respondGroup(w, group, items)
}

type preferencesResponse struct {
	// Preferences: full key → own value, or null = follow the system.
	Preferences map[string]any   `json:"preferences"`
	Effective   service.Regional `json:"effective"`
	System      service.Regional `json:"system"`
	// Options lists the allowed values of every overridable key (time zones are free-form IANA names).
	Options map[string][]string `json:"options"`
}

func (h *Handler) preferencesOf(w http.ResponseWriter, r *http.Request) {
	p, _ := principal.From(r.Context())
	eff, prefs, err := h.svc.EffectiveRegional(r.Context(), p.ID)
	if err != nil {
		h.fail(w, r, "preferences", err)
		return
	}
	sys, err := h.svc.SystemRegional(r.Context())
	if err != nil {
		h.fail(w, r, "preferences", err)
		return
	}
	opts := map[string][]string{}
	for _, d := range h.svc.Registry().UserOverridable() {
		if d.Type == service.TypeEnum {
			opts[d.Key] = d.Enum
		}
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, http.StatusOK, preferencesResponse{Preferences: prefs, Effective: eff, System: sys, Options: opts})
}

func (h *Handler) preferences(w http.ResponseWriter, r *http.Request) { h.preferencesOf(w, r) }

func (h *Handler) putPreferences(w http.ResponseWriter, r *http.Request) {
	p, ok := principal.From(r.Context())
	if !ok {
		httpserver.Error(w, r, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	var in valuesBody
	if !decode(w, r, &in) {
		return
	}
	if _, err := h.svc.UpdatePreferences(r.Context(), p.ID, in.Values, audit.MetaFromRequest(r)); err != nil {
		h.fail(w, r, "update preferences", err)
		return
	}
	h.preferencesOf(w, r)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, what string, err error) {
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		httpserver.FieldError(w, r, http.StatusUnprocessableEntity, CodeValidation, ve.Field, ve.Reason)
	case errors.Is(err, service.ErrUnknownGroup):
		httpserver.Error(w, r, http.StatusNotFound, CodeNotFound, "settings group not found")
	default:
		h.log.ErrorContext(r.Context(), what, "error", err)
		httpserver.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
