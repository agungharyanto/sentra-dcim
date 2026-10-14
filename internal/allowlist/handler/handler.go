// Package handler exposes Allowed IP (v0.3.6, D-078): the admin API under /api/v1/settings/allowed-ips
// (rbac.SettingsManage), the decision endpoint Caddy's forward_auth calls for every public request
// (GET /api/v1/allowlist/check, internal listener only), and the middleware that enforces the same decision on the
// public API listener (second layer behind Caddy).
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/agungharyanto/sentra-dcim/internal/allowlist/service"
	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

// Stable error codes (translated in the UI: errors:code.<code>).
const (
	CodeValidation = "validation_failed"
	CodeNotFound   = "not_found"
	CodeDuplicate  = "duplicate_network"
	CodeLockout    = "lockout_risk"
	CodeNoPending  = "no_pending_change"
	CodeTooMany    = "too_many_entries"
	CodeInvalidReq = "invalid_request"
	CodeNotAllowed = "ip_not_allowed"
)

// AlwaysPublic lists the paths that bypass the allowlist in every mode (monitoring and TLS renewal).
var AlwaysPublic = []string{"/api/v1/health", "/.well-known/acme-challenge/*"}

// Handler serves Allowed IP.
type Handler struct {
	svc *service.Service
	log *slog.Logger
}

// New creates a Handler.
func New(svc *service.Service, log *slog.Logger) *Handler { return &Handler{svc: svc, log: log} }

// Routes registers the admin API (session + settings.manage).
func (h *Handler) Routes(r chi.Router, require func(rbac.Permission) func(http.Handler) http.Handler) {
	manage := require(rbac.SettingsManage)
	r.With(manage).Get("/settings/allowed-ips", h.state)
	r.With(manage).Post("/settings/allowed-ips", h.add)
	r.With(manage).Patch("/settings/allowed-ips/{id}", h.update)
	r.With(manage).Delete("/settings/allowed-ips/{id}", h.remove)
	r.With(manage).Put("/settings/allowed-ips/mode", h.mode)
	r.With(manage).Post("/settings/allowed-ips/confirm", h.confirm)
}

// CheckRoutes registers the forward_auth decision endpoint (internal listener only; no session).
func (h *Handler) CheckRoutes(r chi.Router) { r.Get("/allowlist/check", h.check) }

func clientIP(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return ip.Unmap()
}

// check answers Caddy's forward_auth: 204 = continue, 403 = refuse (body returned to the client). The client IP is
// the X-Real-IP Caddy sets from the TCP connection (realIP middleware, trusted proxy only); the original request is
// in X-Forwarded-Method / X-Forwarded-Uri.
func (h *Handler) check(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	method := r.Header.Get("X-Forwarded-Method")
	path := r.Header.Get("X-Forwarded-Uri")
	if u, err := url.ParseRequestURI(path); err == nil {
		path = u.Path
	} else {
		path = "/"
	}
	d := h.svc.Decide(ip)
	if d.Outside {
		h.svc.RecordOutside(r.Context(), d, ip, method, path, audit.Meta{UserAgent: truncate(r.UserAgent(), 512)})
	}
	if !d.Allow {
		w.Header().Set("Cache-Control", "no-store")
		httpserver.Error(w, r, http.StatusForbidden, CodeNotAllowed, "access from this IP address is not allowed")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Middleware enforces the decision on the public API listener (health stays open). Monitor-mode auditing happens
// in check (Caddy calls it for every request), so this only blocks.
func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/health" {
			next.ServeHTTP(w, r)
			return
		}
		ip := clientIP(r)
		if d := h.svc.Decide(ip); !d.Allow {
			h.svc.RecordOutside(r.Context(), d, ip, r.Method, r.URL.Path, audit.MetaFromRequest(r))
			httpserver.Error(w, r, http.StatusForbidden, CodeNotAllowed, "access from this IP address is not allowed")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type stateResponse struct {
	service.State
	YourIP       string   `json:"your_ip"`
	YourIPInList bool     `json:"your_ip_in_list"`
	AlwaysPublic []string `json:"always_public"`
	// ConfirmWindowSeconds is how long an enforce change waits for "keep" before rolling back to monitor.
	ConfirmWindowSeconds int       `json:"confirm_window_seconds"`
	ServerTime           time.Time `json:"server_time"`
}

func (h *Handler) respond(w http.ResponseWriter, r *http.Request, code int, st service.State) {
	ip := clientIP(r)
	in := false
	for _, e := range st.Entries {
		if p, err := netip.ParsePrefix(e.CIDR); err == nil && ip.IsValid() && p.Contains(ip) {
			in = true
			break
		}
	}
	yours := ""
	if ip.IsValid() {
		yours = ip.String()
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, code, stateResponse{State: st, YourIP: yours, YourIPInList: in, AlwaysPublic: AlwaysPublic,
		ConfirmWindowSeconds: int(service.ConfirmWindow / time.Second), ServerTime: time.Now().UTC()})
}

func (h *Handler) state(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.State(r.Context())
	if err != nil {
		h.fail(w, r, "allowlist state", err)
		return
	}
	h.respond(w, r, http.StatusOK, st)
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		httpserver.Error(w, r, http.StatusBadRequest, CodeInvalidReq, "invalid JSON body")
		return false
	}
	return true
}

func (h *Handler) add(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CIDR  string `json:"cidr"`
		Label string `json:"label"`
	}
	if !decode(w, r, &in) {
		return
	}
	if _, err := h.svc.Add(r.Context(), in.CIDR, in.Label, audit.MetaFromRequest(r)); err != nil {
		h.fail(w, r, "allowlist add", err)
		return
	}
	h.stateAfter(w, r, http.StatusCreated)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.Error(w, r, http.StatusNotFound, CodeNotFound, "entry not found")
		return
	}
	var in struct {
		Label string `json:"label"`
	}
	if !decode(w, r, &in) {
		return
	}
	if _, err := h.svc.UpdateLabel(r.Context(), id, in.Label, audit.MetaFromRequest(r)); err != nil {
		h.fail(w, r, "allowlist update", err)
		return
	}
	h.stateAfter(w, r, http.StatusOK)
}

func (h *Handler) remove(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.Error(w, r, http.StatusNotFound, CodeNotFound, "entry not found")
		return
	}
	if _, err := h.svc.Remove(r.Context(), &id, "", clientIP(r), audit.MetaFromRequest(r)); err != nil {
		h.fail(w, r, "allowlist remove", err)
		return
	}
	h.stateAfter(w, r, http.StatusOK)
}

func (h *Handler) mode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Mode string `json:"mode"`
	}
	if !decode(w, r, &in) {
		return
	}
	st, err := h.svc.SetMode(r.Context(), in.Mode, service.ModeOptions{Requester: clientIP(r)}, audit.MetaFromRequest(r))
	if err != nil {
		h.fail(w, r, "allowlist mode", err)
		return
	}
	h.respond(w, r, http.StatusOK, st)
}

func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	st, err := h.svc.Confirm(r.Context(), clientIP(r), audit.MetaFromRequest(r))
	if err != nil {
		h.fail(w, r, "allowlist confirm", err)
		return
	}
	h.respond(w, r, http.StatusOK, st)
}

func (h *Handler) stateAfter(w http.ResponseWriter, r *http.Request, code int) {
	st, err := h.svc.State(r.Context())
	if err != nil {
		h.fail(w, r, "allowlist state", err)
		return
	}
	h.respond(w, r, code, st)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, what string, err error) {
	var ve *service.ValidationError
	switch {
	case errors.As(err, &ve):
		httpserver.FieldError(w, r, http.StatusUnprocessableEntity, CodeValidation, ve.Field, ve.Reason)
	case errors.Is(err, service.ErrDuplicate):
		httpserver.FieldError(w, r, http.StatusConflict, CodeDuplicate, "cidr", "network already in the list")
	case errors.Is(err, service.ErrNotFound):
		httpserver.Error(w, r, http.StatusNotFound, CodeNotFound, "entry not found")
	case errors.Is(err, service.ErrLockout):
		httpserver.Error(w, r, http.StatusConflict, CodeLockout, "your own IP address would no longer be allowed")
	case errors.Is(err, service.ErrNoPending):
		httpserver.Error(w, r, http.StatusConflict, CodeNoPending, "no enforce change is waiting for confirmation")
	case errors.Is(err, service.ErrTooMany):
		httpserver.Error(w, r, http.StatusUnprocessableEntity, CodeTooMany, "too many entries")
	default:
		h.log.ErrorContext(r.Context(), what, "error", err)
		httpserver.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	for n > 0 && (s[n]&0xC0) == 0x80 {
		n--
	}
	return strings.Clone(s[:n])
}
