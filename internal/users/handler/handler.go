// Package handler exposes user administration under /api/v1/users and /api/v1/roles (v0.3.2). Every route
// requires a permission (rbac.UsersRead, rbac.UsersManage, rbac.RolesRead).
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
	"github.com/agungharyanto/sentra-dcim/internal/users/service"
)

// Stable error codes (translated in the UI: errors:code.<code>).
const (
	CodeValidation    = "validation_failed"
	CodeNotFound      = "not_found"
	CodeUsernameTaken = "username_taken"
	CodeLastAdmin     = "last_admin"
	CodeSelfAction    = "self_action"
	CodeInvalidJSON   = "invalid_request"
)

// Handler serves user administration.
type Handler struct {
	svc *service.Service
	log *slog.Logger
}

// New creates a Handler.
func New(svc *service.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

// Routes registers the endpoints; require returns the permission middleware (auth handler Require).
func (h *Handler) Routes(r chi.Router, require func(rbac.Permission) func(http.Handler) http.Handler) {
	read, manage := require(rbac.UsersRead), require(rbac.UsersManage)
	r.With(read).Get("/users", h.list)
	r.With(manage).Post("/users", h.create)
	r.With(read).Get("/users/{id}", h.get)
	r.With(manage).Patch("/users/{id}", h.update)
	r.With(manage).Put("/users/{id}/roles", h.setRoles)
	r.With(manage).Post("/users/{id}/disable", h.status(service.StatusDisabled))
	r.With(manage).Post("/users/{id}/enable", h.status(service.StatusActive))
	r.With(manage).Post("/users/{id}/password-reset", h.resetPassword)
	r.With(require(rbac.RolesRead)).Get("/roles", h.roles)
}

type listResponse struct {
	Users  []service.User `json:"users"`
	Total  int64          `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, err1 := atoiDefault(q.Get("limit"), service.DefaultLimit)
	offset, err2 := atoiDefault(q.Get("offset"), 0)
	if err1 != nil || err2 != nil {
		httpserver.FieldError(w, r, http.StatusBadRequest, CodeValidation, "limit", "limit and offset must be integers")
		return
	}
	users, total, err := h.svc.List(r.Context(), service.Filter{Query: q.Get("q"), Status: q.Get("status"), Role: q.Get("role"),
		Limit: limit, Offset: offset})
	if err != nil {
		h.fail(w, r, "list users", err)
		return
	}
	if limit <= 0 {
		limit = service.DefaultLimit
	}
	httpserver.JSON(w, http.StatusOK, listResponse{Users: users, Total: total, Limit: limit, Offset: offset})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	u, err := h.svc.Get(r.Context(), id)
	if err != nil {
		h.fail(w, r, "get user", err)
		return
	}
	httpserver.JSON(w, http.StatusOK, u)
}

type withPassword struct {
	User              *service.User `json:"user,omitempty"`
	TemporaryPassword string        `json:"temporary_password"` //nolint:gosec // returned once to the Admin by design
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username    string   `json:"username"`
		DisplayName string   `json:"display_name"`
		Email       string   `json:"email"`
		Roles       []string `json:"roles"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, temp, err := h.svc.Create(r.Context(), service.CreateInput{Username: in.Username, DisplayName: in.DisplayName,
		Email: in.Email, Roles: in.Roles}, audit.MetaFromRequest(r))
	if err != nil {
		h.fail(w, r, "create user", err)
		return
	}
	noStore(w)
	httpserver.JSON(w, http.StatusCreated, withPassword{User: &u, TemporaryPassword: temp})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	var in struct {
		DisplayName *string `json:"display_name"`
		Email       *string `json:"email"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, err := h.svc.UpdateProfile(r.Context(), id, service.ProfileInput{DisplayName: in.DisplayName, Email: in.Email},
		audit.MetaFromRequest(r))
	if err != nil {
		h.fail(w, r, "update user", err)
		return
	}
	httpserver.JSON(w, http.StatusOK, u)
}

func (h *Handler) setRoles(w http.ResponseWriter, r *http.Request) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	var in struct {
		Roles []string `json:"roles"`
	}
	if !decode(w, r, &in) {
		return
	}
	u, err := h.svc.SetRoles(r.Context(), id, in.Roles, audit.MetaFromRequest(r))
	if err != nil {
		h.fail(w, r, "set roles", err)
		return
	}
	httpserver.JSON(w, http.StatusOK, u)
}

func (h *Handler) status(status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := userID(w, r)
		if !ok {
			return
		}
		u, err := h.svc.SetStatus(r.Context(), id, status, audit.MetaFromRequest(r))
		if err != nil {
			h.fail(w, r, "set status", err)
			return
		}
		httpserver.JSON(w, http.StatusOK, u)
	}
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	id, ok := userID(w, r)
	if !ok {
		return
	}
	temp, err := h.svc.ResetPassword(r.Context(), id, audit.MetaFromRequest(r))
	if err != nil {
		h.fail(w, r, "reset password", err)
		return
	}
	noStore(w)
	httpserver.JSON(w, http.StatusOK, withPassword{TemporaryPassword: temp})
}

func (h *Handler) roles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.svc.Roles(r.Context())
	if err != nil {
		h.fail(w, r, "list roles", err)
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{"roles": roles, "permissions": rbac.Strings(rbac.All)})
}

// fail maps service errors to responses.
func (h *Handler) fail(w http.ResponseWriter, r *http.Request, what string, err error) {
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		httpserver.FieldError(w, r, http.StatusUnprocessableEntity, CodeValidation, ve.Field, ve.Reason)
	case errors.Is(err, service.ErrNotFound):
		httpserver.Error(w, r, http.StatusNotFound, CodeNotFound, "user not found")
	case errors.Is(err, service.ErrUsernameTaken):
		httpserver.FieldError(w, r, http.StatusConflict, CodeUsernameTaken, "username", "username already exists")
	case errors.Is(err, service.ErrLastAdmin):
		httpserver.Error(w, r, http.StatusConflict, CodeLastAdmin, err.Error())
	case errors.Is(err, service.ErrSelf):
		httpserver.Error(w, r, http.StatusConflict, CodeSelfAction, err.Error())
	default:
		h.log.ErrorContext(r.Context(), what, "error", err)
		httpserver.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func userID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.Error(w, r, http.StatusNotFound, CodeNotFound, "user not found")
		return uuid.Nil, false
	}
	return id, true
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		httpserver.Error(w, r, http.StatusBadRequest, CodeInvalidJSON, "invalid JSON body")
		return false
	}
	return true
}

func atoiDefault(s string, def int) (int, error) {
	if s == "" {
		return def, nil
	}
	return strconv.Atoi(s)
}

// noStore keeps responses carrying a temporary password out of every cache.
func noStore(w http.ResponseWriter) { w.Header().Set("Cache-Control", "no-store") }
