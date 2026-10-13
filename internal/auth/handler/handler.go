// Package handler exposes authentication under /api/v1/auth and provides the authentication, CSRF and
// password-change middlewares used by the default-deny router (ADR-003).
package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/auth/password"
	"github.com/agungharyanto/sentra-dcim/internal/auth/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
)

// Cookie and header names. `__Host-` forces Secure, Path=/ and no Domain attribute.
const (
	SessionCookie = "__Host-sentra_session"
	CSRFCookie    = "__Host-sentra_csrf"
	CSRFHeader    = "X-Sentra-CSRF"
)

// Stable error codes (translated in the UI: errors:code.<code>).
const (
	CodeUnauthenticated        = "unauthenticated"
	CodeInvalidCredentials     = "invalid_credentials" //nolint:gosec // error code, not a credential
	CodeRateLimited            = "rate_limited"
	CodeCSRF                   = "csrf_failed"
	CodePasswordChangeRequired = "password_change_required"
	CodePasswordPolicy         = "password_policy"
	CodeInvalidCurrentPassword = "invalid_current_password"
	CodeInvalidRequest         = "invalid_request"
	CodeUnavailable            = "auth_unavailable"
)

// Handler serves the auth endpoints.
type Handler struct {
	svc     *service.Service
	log     *slog.Logger
	origins []string
}

// New creates a Handler. origins are the exact origins accepted for unsafe requests.
func New(svc *service.Service, log *slog.Logger, origins []string) *Handler {
	return &Handler{svc: svc, log: log, origins: origins}
}

// PublicRoutes registers the only unauthenticated auth endpoint (login).
func (h *Handler) PublicRoutes(r chi.Router) {
	r.With(h.CSRF).Post("/auth/login", h.login)
}

// Routes registers the authenticated auth endpoints (mounted behind Authenticate + CSRF + PasswordChangeGuard).
func (h *Handler) Routes(r chi.Router) {
	r.Post("/auth/logout", h.logout)
	r.Get("/auth/me", h.me)
	r.Post("/auth/password", h.changePassword)
	r.Get("/auth/sessions", h.sessions)
	r.Delete("/auth/sessions/{id}", h.revokeSession)
}

type sessionKey struct{}

// SessionFrom returns the authenticated session of the request.
func SessionFrom(ctx context.Context) (service.Session, bool) {
	s, ok := ctx.Value(sessionKey{}).(service.Session)
	return s, ok
}

// Authenticate requires a valid session cookie (default-deny). It stores the session and principal in the
// context; an invalid or expired cookie is cleared.
func (h *Handler) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(SessionCookie)
		if err != nil || c.Value == "" {
			httpserver.Error(w, r, http.StatusUnauthorized, CodeUnauthenticated, "authentication required")
			return
		}
		if h.svc == nil {
			httpserver.Error(w, r, http.StatusUnauthorized, CodeUnauthenticated, "authentication required")
			return
		}
		sess, err := h.svc.Authenticate(r.Context(), c.Value)
		if errors.Is(err, service.ErrUnauthenticated) {
			clearCookies(w)
			httpserver.Error(w, r, http.StatusUnauthorized, CodeUnauthenticated, "authentication required")
			return
		}
		if err != nil {
			h.log.ErrorContext(r.Context(), "authenticate", "error", err)
			httpserver.Error(w, r, http.StatusServiceUnavailable, CodeUnavailable, "authentication temporarily unavailable")
			return
		}
		ctx := context.WithValue(r.Context(), sessionKey{}, sess)
		ctx = principal.With(ctx, sess.Principal)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CSRF protects unsafe methods: the Origin (or Referer) must be an allowed origin and X-Sentra-CSRF must be
// present — and, once a session exists, equal to the CSRF cookie (double-submit).
func (h *Handler) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if !h.originAllowed(r) {
			httpserver.Error(w, r, http.StatusForbidden, CodeCSRF, "request origin not allowed")
			return
		}
		hdr := r.Header.Get(CSRFHeader)
		if hdr == "" {
			httpserver.Error(w, r, http.StatusForbidden, CodeCSRF, "missing CSRF header")
			return
		}
		if _, authed := SessionFrom(r.Context()); authed {
			c, err := r.Cookie(CSRFCookie)
			if err != nil || subtle.ConstantTimeCompare([]byte(c.Value), []byte(hdr)) != 1 {
				httpserver.Error(w, r, http.StatusForbidden, CodeCSRF, "CSRF token mismatch")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) originAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" || origin == "null" {
		ref, err := url.Parse(r.Header.Get("Referer"))
		if err != nil || ref.Host == "" {
			return false
		}
		origin = ref.Scheme + "://" + ref.Host
	}
	return slices.Contains(h.origins, strings.ToLower(strings.TrimRight(origin, "/")))
}

// passwordChangeAllowed are the only routes usable while must_change_password is set.
var passwordChangeAllowed = map[string]bool{
	"GET /api/v1/auth/me":        true,
	"POST /api/v1/auth/password": true,
	"POST /api/v1/auth/logout":   true,
}

// PasswordChangeGuard blocks everything except me/password/logout while the user must change the password.
func PasswordChangeGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s, ok := SessionFrom(r.Context()); ok && s.MustChangePassword && !passwordChangeAllowed[r.Method+" "+r.URL.Path] {
			httpserver.Error(w, r, http.StatusForbidden, CodePasswordChangeRequired, "password change required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

type meResponse struct {
	User    service.User        `json:"user"`
	Session service.SessionInfo `json:"session"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &in) {
		return
	}
	old := ""
	if c, err := r.Cookie(SessionCookie); err == nil {
		old = c.Value
	}
	res, err := h.svc.Login(r.Context(), in.Username, in.Password, old, audit.MetaFromRequest(r))
	var rl *service.RateLimitError
	switch {
	case errors.As(err, &rl):
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(rl.RetryAfter.Seconds()))))
		httpserver.Error(w, r, http.StatusTooManyRequests, CodeRateLimited, "too many login attempts, try again later")
		return
	case errors.Is(err, service.ErrInvalidCredentials):
		// One generic answer for unknown user, wrong password, locked and disabled accounts.
		httpserver.Error(w, r, http.StatusUnauthorized, CodeInvalidCredentials, "invalid username or password")
		return
	case errors.Is(err, service.ErrUnavailable):
		httpserver.Error(w, r, http.StatusServiceUnavailable, CodeUnavailable, "authentication temporarily unavailable")
		return
	case err != nil:
		h.log.ErrorContext(r.Context(), "login", "error", err)
		httpserver.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	setSessionCookies(w, res.Token, res.CSRF)
	httpserver.JSON(w, http.StatusOK, meResponse{User: res.Session.User, Session: res.Session.Info})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r.Context())
	if err := h.svc.Logout(r.Context(), sess, audit.MetaFromRequest(r)); err != nil {
		h.log.ErrorContext(r.Context(), "logout", "error", err)
	}
	clearCookies(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r.Context())
	httpserver.JSON(w, http.StatusOK, meResponse{User: sess.User, Session: sess.Info})
}

func (h *Handler) changePassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Current string `json:"current_password"`
		New     string `json:"new_password"`
	}
	if !decode(w, r, &in) {
		return
	}
	sess, _ := SessionFrom(r.Context())
	err := h.svc.ChangePassword(r.Context(), sess, in.Current, in.New, audit.MetaFromRequest(r))
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		httpserver.Error(w, r, http.StatusBadRequest, CodeInvalidCurrentPassword, "current password is incorrect")
	case errors.Is(err, service.ErrPolicy):
		httpserver.JSON(w, http.StatusUnprocessableEntity, map[string]any{"error": map[string]any{
			"code": CodePasswordPolicy, "message": "new password must be 12 to 128 characters and differ from the current one",
			"min_length": password.MinLength, "max_length": password.MaxLength}})
	case err != nil:
		h.log.ErrorContext(r.Context(), "change password", "error", err)
		httpserver.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *Handler) sessions(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r.Context())
	list, err := h.svc.ListSessions(r.Context(), sess)
	if err != nil {
		h.log.ErrorContext(r.Context(), "list sessions", "error", err)
		httpserver.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{"sessions": list})
}

func (h *Handler) revokeSession(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httpserver.Error(w, r, http.StatusNotFound, "not_found", "session not found")
		return
	}
	err = h.svc.RevokeSession(r.Context(), sess, id, audit.MetaFromRequest(r))
	switch {
	case errors.Is(err, service.ErrNotFound):
		httpserver.Error(w, r, http.StatusNotFound, "not_found", "session not found")
	case err != nil:
		h.log.ErrorContext(r.Context(), "revoke session", "error", err)
		httpserver.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	default:
		if id == sess.Info.ID {
			clearCookies(w)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		httpserver.Error(w, r, http.StatusBadRequest, CodeInvalidRequest, "invalid JSON body")
		return false
	}
	return true
}

// setSessionCookies writes the session (HttpOnly) and CSRF cookies as browser-session cookies (no Max-Age: closing
// the browser ends them; the server enforces the idle/absolute limits).
func setSessionCookies(w http.ResponseWriter, token, csrf string) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: token, Path: "/", Secure: true, HttpOnly: true,
		SameSite: http.SameSiteStrictMode})
	// Not HttpOnly by design: the UI reads it to send X-Sentra-CSRF (double-submit). It grants nothing on its own.
	http.SetCookie(w, &http.Cookie{Name: CSRFCookie, Value: csrf, Path: "/", Secure: true, //nolint:gosec // double-submit token
		SameSite: http.SameSiteStrictMode})
}

func clearCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Value: "", Path: "/", Secure: true, HttpOnly: true,
		SameSite: http.SameSiteStrictMode, MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: CSRFCookie, Value: "", Path: "/", Secure: true, //nolint:gosec // double-submit token
		SameSite: http.SameSiteStrictMode, MaxAge: -1})
}
