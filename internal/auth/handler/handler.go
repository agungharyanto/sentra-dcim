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
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/auth/password"
	"github.com/agungharyanto/sentra-dcim/internal/auth/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
	settings "github.com/agungharyanto/sentra-dcim/internal/settings/service"
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
	CodePasswordReused         = "password_reused"
	CodeInvalidCurrentPassword = "invalid_current_password"
	CodeInvalidRequest         = "invalid_request"
	CodeUnavailable            = "auth_unavailable"
	CodeForbidden              = "forbidden"
	CodeMFARequired            = "mfa_required"
	CodeInvalidMFACode         = "invalid_mfa_code"
	CodeMFAEnabled             = "mfa_already_enabled"
	CodeMFANotEnrolled         = "mfa_not_enrolled"
)

// Handler serves the auth endpoints.
type Handler struct {
	svc      *service.Service
	log      *slog.Logger
	origins  []string
	regional RegionalProvider
}

// RegionalProvider resolves a user's effective regional settings (internal/settings, v0.3.5).
type RegionalProvider interface {
	EffectiveRegional(ctx context.Context, userID uuid.UUID) (settings.Regional, map[string]any, error)
}

// WithRegional makes /auth/me and the login response carry the effective regional settings.
func (h *Handler) WithRegional(p RegionalProvider) *Handler { h.regional = p; return h }

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
	r.Post("/auth/activity", h.activity)
	r.Get("/auth/me", h.me)
	r.Post("/auth/password", h.changePassword)
	r.Get("/auth/sessions", h.sessions)
	r.Delete("/auth/sessions/{id}", h.revokeSession)
	r.Get("/auth/mfa", h.mfaStatus)
	r.Post("/auth/mfa/enrol", h.mfaEnrol)
	r.Post("/auth/mfa/enrol/confirm", h.mfaConfirm)
	r.Post("/auth/mfa/verify", h.mfaVerify)
	r.Post("/auth/mfa/recovery-codes", h.mfaRecoveryCodes)
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
		// Only write requests count as user activity here; reads (and background polling) never extend the idle
		// deadline — the UI sends POST /auth/activity on real interaction (D-069).
		active := r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions
		sess, err := h.svc.Authenticate(r.Context(), c.Value, clientAddr(r), active)
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

// Identify is the optional variant of Authenticate for public routes whose answer depends on the caller (e.g.
// /version details): a valid session sets the principal, anything else continues anonymously. It never fails.
func (h *Handler) Identify(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(SessionCookie); err == nil && c.Value != "" && h.svc != nil {
			// Only a session that passed MFA identifies the caller.
			if sess, err := h.svc.Authenticate(r.Context(), c.Value, clientAddr(r), false); err == nil && sess.MFAVerified {
				ctx := context.WithValue(r.Context(), sessionKey{}, sess)
				r = r.WithContext(principal.With(ctx, sess.Principal))
			}
		}
		next.ServeHTTP(w, r)
	})
}

// Require answers 403 "forbidden" (and records authz.denied) unless the principal holds perm. Mount it on each
// route that needs a permission, behind Authenticate; routes without it are open to every signed-in user and must be
// listed in api.SelfServiceRoutes (enforced by the integration test TestPermissionDefaultDeny).
func (h *Handler) Require(perm rbac.Permission) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := principal.From(r.Context())
			if !ok {
				httpserver.Error(w, r, http.StatusUnauthorized, CodeUnauthenticated, "authentication required")
				return
			}
			if !p.Can(perm) {
				if h.svc != nil {
					route := r.Method + " " + chi.RouteContext(r.Context()).RoutePattern()
					h.svc.RecordDenied(r.Context(), perm, route, audit.MetaFromRequest(r))
				}
				httpserver.Error(w, r, http.StatusForbidden, CodeForbidden, "permission denied")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
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
			h.csrfDenied(w, r, "origin", "request origin not allowed")
			return
		}
		hdr := r.Header.Get(CSRFHeader)
		if hdr == "" {
			h.csrfDenied(w, r, "missing_header", "missing CSRF header")
			return
		}
		if _, authed := SessionFrom(r.Context()); authed {
			c, err := r.Cookie(CSRFCookie)
			if err != nil || subtle.ConstantTimeCompare([]byte(c.Value), []byte(hdr)) != 1 {
				h.csrfDenied(w, r, "token_mismatch", "CSRF token mismatch")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// csrfDenied answers 403 csrf_failed and audits it (never the token values).
func (h *Handler) csrfDenied(w http.ResponseWriter, r *http.Request, reason, msg string) {
	if h.svc != nil {
		origin := r.Header.Get("Origin")
		if origin == "" {
			if ref, err := url.Parse(r.Header.Get("Referer")); err == nil {
				origin = ref.Scheme + "://" + ref.Host
			}
		}
		h.svc.RecordCSRFDenied(r.Context(), reason, r.Method+" "+r.URL.Path, origin, audit.MetaFromRequest(r))
	}
	httpserver.Error(w, r, http.StatusForbidden, CodeCSRF, msg)
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

// mfaPendingAllowed are the only routes usable before the session passed MFA (D-062): who am I, sign out, and the
// MFA steps themselves.
var mfaPendingAllowed = map[string]bool{
	"GET /api/v1/auth/me":                 true,
	"POST /api/v1/auth/logout":            true,
	"GET /api/v1/auth/mfa":                true,
	"POST /api/v1/auth/mfa/enrol":         true,
	"POST /api/v1/auth/mfa/enrol/confirm": true,
	"POST /api/v1/auth/mfa/verify":        true,
}

// MFAGuard blocks everything except mfaPendingAllowed until the session passed MFA (403 "mfa_required").
func MFAGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s, ok := SessionFrom(r.Context()); ok && !s.MFAVerified && !mfaPendingAllowed[r.Method+" "+r.URL.Path] {
			httpserver.Error(w, r, http.StatusForbidden, CodeMFARequired, "multi-factor authentication required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// passwordChangeAllowed are the only routes usable while must_change_password is set (the MFA steps come first).
var passwordChangeAllowed = map[string]bool{
	"GET /api/v1/auth/me":                 true,
	"POST /api/v1/auth/password":          true,
	"POST /api/v1/auth/logout":            true,
	"GET /api/v1/auth/mfa":                true,
	"POST /api/v1/auth/mfa/enrol":         true,
	"POST /api/v1/auth/mfa/enrol/confirm": true,
	"POST /api/v1/auth/mfa/verify":        true,
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
	// Regional is the effective regional configuration (user preference over system setting); omitted only when
	// no provider is configured (tests).
	Regional *settings.Regional `json:"regional,omitempty"`
	// ServerTime lets the UI count down to the session deadlines independent of the client clock (v0.3.7).
	ServerTime time.Time `json:"server_time"`
}

// clientAddr is the client IP resolved by the realIP middleware (invalid when unknown).
func clientAddr(r *http.Request) netip.Addr {
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

type activityResponse struct {
	Session    service.SessionInfo `json:"session"`
	ServerTime time.Time           `json:"server_time"`
}

// activity extends the idle deadline on real user interaction (D-069) and returns the new deadlines.
func (h *Handler) activity(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r.Context())
	info, err := h.svc.Activity(r.Context(), sess)
	if err != nil {
		h.log.ErrorContext(r.Context(), "session activity", "error", err)
		httpserver.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, http.StatusOK, activityResponse{Session: info, ServerTime: time.Now().UTC()})
}

func (h *Handler) meOf(ctx context.Context, user service.User, info service.SessionInfo) meResponse {
	resp := meResponse{User: user, Session: info, ServerTime: time.Now().UTC()}
	if h.regional != nil {
		if reg, _, err := h.regional.EffectiveRegional(ctx, user.ID); err == nil {
			resp.Regional = &reg
		} else {
			h.log.ErrorContext(ctx, "effective regional settings", "error", err)
		}
	}
	return resp
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
	httpserver.JSON(w, http.StatusOK, h.meOf(r.Context(), res.Session.User, res.Session.Info))
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
	httpserver.JSON(w, http.StatusOK, h.meOf(r.Context(), sess.User, sess.Info))
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
	case errors.Is(err, service.ErrPasswordReused):
		httpserver.FieldError(w, r, http.StatusUnprocessableEntity, CodePasswordReused, "new_password", "password used recently")
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

func (h *Handler) mfaStatus(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r.Context())
	st, err := h.svc.MFAStatus(r.Context(), sess)
	if err != nil {
		h.log.ErrorContext(r.Context(), "mfa status", "error", err)
		httpserver.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	httpserver.JSON(w, http.StatusOK, st)
}

func (h *Handler) mfaEnrol(w http.ResponseWriter, r *http.Request) {
	sess, _ := SessionFrom(r.Context())
	en, err := h.svc.StartEnrolment(r.Context(), sess, audit.MetaFromRequest(r))
	if err != nil {
		h.mfaError(w, r, "mfa enrol", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, http.StatusOK, en)
}

type codeBody struct {
	Code string `json:"code"`
}

func (h *Handler) mfaConfirm(w http.ResponseWriter, r *http.Request) {
	var in codeBody
	if !decode(w, r, &in) {
		return
	}
	sess, _ := SessionFrom(r.Context())
	codes, err := h.svc.ConfirmEnrolment(r.Context(), sess, in.Code, audit.MetaFromRequest(r))
	if err != nil {
		h.mfaError(w, r, "mfa confirm", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

func (h *Handler) mfaVerify(w http.ResponseWriter, r *http.Request) {
	var in codeBody
	if !decode(w, r, &in) {
		return
	}
	sess, _ := SessionFrom(r.Context())
	if err := h.svc.VerifyMFA(r.Context(), sess, in.Code, audit.MetaFromRequest(r)); err != nil {
		h.mfaError(w, r, "mfa verify", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) mfaRecoveryCodes(w http.ResponseWriter, r *http.Request) {
	var in codeBody
	if !decode(w, r, &in) {
		return
	}
	sess, _ := SessionFrom(r.Context())
	codes, err := h.svc.RegenerateRecoveryCodes(r.Context(), sess, in.Code, audit.MetaFromRequest(r))
	if err != nil {
		h.mfaError(w, r, "mfa recovery codes", err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, http.StatusOK, map[string]any{"recovery_codes": codes})
}

// mfaError maps MFA errors. A wrong code is 401 like a wrong password (counted by the login jail).
func (h *Handler) mfaError(w http.ResponseWriter, r *http.Request, what string, err error) {
	var rl *service.RateLimitError
	switch {
	case errors.As(err, &rl):
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(rl.RetryAfter.Seconds()))))
		httpserver.Error(w, r, http.StatusTooManyRequests, CodeRateLimited, "too many attempts, try again later")
	case errors.Is(err, service.ErrInvalidMFACode):
		httpserver.Error(w, r, http.StatusUnauthorized, CodeInvalidMFACode, "invalid authentication code")
	case errors.Is(err, service.ErrMFAEnabled):
		httpserver.Error(w, r, http.StatusConflict, CodeMFAEnabled, "MFA is already active")
	case errors.Is(err, service.ErrMFANotEnrolled):
		httpserver.Error(w, r, http.StatusConflict, CodeMFANotEnrolled, "MFA is not set up")
	case errors.Is(err, service.ErrUnavailable):
		httpserver.Error(w, r, http.StatusServiceUnavailable, CodeUnavailable, "authentication temporarily unavailable")
	default:
		h.log.ErrorContext(r.Context(), what, "error", err)
		httpserver.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
