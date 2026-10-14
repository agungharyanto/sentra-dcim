// Package api assembles the /api/v1 routes of both listeners. Everything is default-deny: a route is reachable
// without a session only when it is registered in the public group below (see PublicRoutes and api_test.go).
package api

import (
	"github.com/go-chi/chi/v5"

	audithandler "github.com/agungharyanto/sentra-dcim/internal/audit/handler"
	authhandler "github.com/agungharyanto/sentra-dcim/internal/auth/handler"
	helphandler "github.com/agungharyanto/sentra-dcim/internal/help/handler"
	systemhandler "github.com/agungharyanto/sentra-dcim/internal/system/handler"
	uihandler "github.com/agungharyanto/sentra-dcim/internal/ui/handler"
	usershandler "github.com/agungharyanto/sentra-dcim/internal/users/handler"
)

// Modules are the HTTP modules mounted under /api/v1.
type Modules struct {
	System *systemhandler.Handler
	UI     *uihandler.Handler
	Help   *helphandler.Handler
	Auth   *authhandler.Handler
	Users  *usershandler.Handler
	Audit  *audithandler.Handler
}

// PublicRoutes is the allowlist of "METHOD /path" reachable without a session on the public listener.
var PublicRoutes = []string{
	"GET /api/v1/health",
	"GET /api/v1/version",
	"POST /api/v1/auth/login",
}

// InternalPublicRoutes extends PublicRoutes on the internal (loopback-only) listener for ops scripts.
var InternalPublicRoutes = append(append([]string{}, PublicRoutes...), "GET /api/v1/ready")

// SelfServiceRoutes are the authenticated routes open to every signed-in user (no permission). Every other route
// behind the session must require a permission (Auth.Require) — test/integration TestPermissionDefaultDeny signs in
// without roles and expects 403 from everything not listed here.
var SelfServiceRoutes = []string{
	"POST /api/v1/auth/logout",
	"GET /api/v1/auth/me",
	"POST /api/v1/auth/password",
	"GET /api/v1/auth/sessions",
	"DELETE /api/v1/auth/sessions/{id}",
	"GET /api/v1/auth/mfa",
	"POST /api/v1/auth/mfa/enrol",
	"POST /api/v1/auth/mfa/enrol/confirm",
	"POST /api/v1/auth/mfa/verify",
	"POST /api/v1/auth/mfa/recovery-codes",
	"GET /api/v1/ui/bootstrap",
	"GET /api/v1/help/index",
	"GET /api/v1/help/pages/*",
	"GET /api/v1/help/openapi",
	"GET /api/v1/help/assets/{name}",
	"GET /api/v1/ready", // public listener only: a signed-in check of dependency status
}

// AuditedRoutes maps every state-changing route (POST/PUT/PATCH/DELETE) to the audit actions it records (success
// or failure). test/integration TestAuditCoverage fails when an unsafe route is missing here, and checks that a
// realistic session of every route really writes each listed action. Refusals are audited centrally: 403 by
// Auth.Require (authz.denied), CSRF by Auth.CSRF (security.csrf_denied), both aggregated when repeated.
var AuditedRoutes = map[string][]string{
	"POST /api/v1/auth/login":                {"auth.login.password_ok", "auth.login.failure"},
	"POST /api/v1/auth/logout":               {"auth.logout"},
	"POST /api/v1/auth/password":             {"auth.password.change"},
	"DELETE /api/v1/auth/sessions/{id}":      {"auth.session.revoke"},
	"POST /api/v1/auth/mfa/enrol":            {"auth.mfa.enrol_start"},
	"POST /api/v1/auth/mfa/enrol/confirm":    {"auth.mfa.enrol", "auth.login.success"},
	"POST /api/v1/auth/mfa/verify":           {"auth.mfa.verify", "auth.login.success"},
	"POST /api/v1/auth/mfa/recovery-codes":   {"auth.mfa.recovery_codes"},
	"POST /api/v1/users":                     {"user.create"},
	"PATCH /api/v1/users/{id}":               {"user.update"},
	"PUT /api/v1/users/{id}/roles":           {"user.roles.update"},
	"POST /api/v1/users/{id}/disable":        {"user.disable"},
	"POST /api/v1/users/{id}/enable":         {"user.enable"},
	"POST /api/v1/users/{id}/password-reset": {"user.password.reset"},
	"POST /api/v1/users/{id}/mfa-reset":      {"user.mfa.reset"},
	"POST /api/v1/audit/integrity/verify":    {"audit.verify"},
}

// Mount returns the /api/v1 mount function. internal selects the loopback listener variant.
func Mount(m Modules, internal bool) func(r chi.Router) {
	return func(r chi.Router) {
		// Public group — keep in sync with PublicRoutes / InternalPublicRoutes.
		r.Group(func(r chi.Router) {
			m.System.PublicRoutes(r, m.Auth.Identify)
			m.Auth.PublicRoutes(r)
			if internal {
				m.System.OpsRoutes(r)
			}
		})
		// Everything else requires a session, passes CSRF for unsafe methods, and is blocked until the session
		// passed MFA and while the user must change the password.
		r.Group(func(r chi.Router) {
			r.Use(m.Auth.Authenticate, m.Auth.CSRF, authhandler.MFAGuard, authhandler.PasswordChangeGuard)
			if !internal {
				m.System.OpsRoutes(r)
			}
			m.Auth.Routes(r)
			m.UI.Routes(r)
			m.Help.Routes(r)
			m.Users.Routes(r, m.Auth.Require)
			m.Audit.Routes(r, m.Auth.Require)
		})
	}
}
