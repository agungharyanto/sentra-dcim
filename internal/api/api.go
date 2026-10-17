// Package api assembles the /api/v1 routes of both listeners. Everything is default-deny: a route is reachable
// without a session only when it is registered in the public group below (see PublicRoutes and api_test.go).
package api

import (
	"github.com/go-chi/chi/v5"

	allowlisthandler "github.com/agungharyanto/sentra-dcim/internal/allowlist/handler"
	audithandler "github.com/agungharyanto/sentra-dcim/internal/audit/handler"
	authhandler "github.com/agungharyanto/sentra-dcim/internal/auth/handler"
	facilityhandler "github.com/agungharyanto/sentra-dcim/internal/facility/handler"
	helphandler "github.com/agungharyanto/sentra-dcim/internal/help/handler"
	settingshandler "github.com/agungharyanto/sentra-dcim/internal/settings/handler"
	systemhandler "github.com/agungharyanto/sentra-dcim/internal/system/handler"
	uihandler "github.com/agungharyanto/sentra-dcim/internal/ui/handler"
	usershandler "github.com/agungharyanto/sentra-dcim/internal/users/handler"
)

// Modules are the HTTP modules mounted under /api/v1.
type Modules struct {
	System    *systemhandler.Handler
	UI        *uihandler.Handler
	Help      *helphandler.Handler
	Auth      *authhandler.Handler
	Users     *usershandler.Handler
	Audit     *audithandler.Handler
	Settings  *settingshandler.Handler
	Allowlist *allowlisthandler.Handler
	Facility  *facilityhandler.Handler
}

// PublicRoutes is the allowlist of "METHOD /path" reachable without a session on the public listener.
var PublicRoutes = []string{
	"GET /api/v1/health",
	"GET /api/v1/version",
	"POST /api/v1/auth/login",
}

// InternalPublicRoutes extends PublicRoutes on the internal (loopback-only) listener for ops scripts.
var InternalPublicRoutes = append(append([]string{}, PublicRoutes...), "GET /api/v1/ready", "GET /api/v1/allowlist/check")

// SelfServiceRoutes are the authenticated routes open to every signed-in user (no permission). Every other route
// behind the session must require a permission (Auth.Require) — test/integration TestPermissionDefaultDeny signs in
// without roles and expects 403 from everything not listed here.
var SelfServiceRoutes = []string{
	"POST /api/v1/auth/logout",
	"POST /api/v1/auth/activity",
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
	"GET /api/v1/me/preferences",
	"PUT /api/v1/me/preferences",
	"GET /api/v1/ready", // public listener only: a signed-in check of dependency status
}

// AuditedRoutes maps every state-changing route (POST/PUT/PATCH/DELETE) to the audit actions it records (success
// or failure). test/integration TestAuditCoverage fails when an unsafe route is missing here, and checks that a
// realistic session of every route really writes each listed action. Refusals are audited centrally: 403 by
// Auth.Require (authz.denied), CSRF by Auth.CSRF (security.csrf_denied), both aggregated when repeated.
var AuditedRoutes = map[string][]string{
	"POST /api/v1/auth/login":                   {"auth.login.password_ok", "auth.login.failure"},
	"POST /api/v1/auth/logout":                  {"auth.logout"},
	"POST /api/v1/auth/password":                {"auth.password.change"},
	"DELETE /api/v1/auth/sessions/{id}":         {"auth.session.revoke"},
	"POST /api/v1/auth/mfa/enrol":               {"auth.mfa.enrol_start"},
	"POST /api/v1/auth/mfa/enrol/confirm":       {"auth.mfa.enrol", "auth.login.success"},
	"POST /api/v1/auth/mfa/verify":              {"auth.mfa.verify", "auth.login.success"},
	"POST /api/v1/auth/mfa/recovery-codes":      {"auth.mfa.recovery_codes"},
	"POST /api/v1/users":                        {"user.create"},
	"PATCH /api/v1/users/{id}":                  {"user.update"},
	"PUT /api/v1/users/{id}/roles":              {"user.roles.update"},
	"POST /api/v1/users/{id}/disable":           {"user.disable"},
	"POST /api/v1/users/{id}/enable":            {"user.enable"},
	"POST /api/v1/users/{id}/password-reset":    {"user.password.reset"},
	"POST /api/v1/users/{id}/mfa-reset":         {"user.mfa.reset"},
	"PUT /api/v1/users/{id}/kiosk":              {"user.kiosk"},
	"POST /api/v1/audit/integrity/verify":       {"audit.verify"},
	"PUT /api/v1/settings/{group}":              {"settings.update"},
	"PUT /api/v1/me/preferences":                {"user.preferences.update"},
	"POST /api/v1/settings/allowed-ips":         {"allowlist.add"},
	"PATCH /api/v1/settings/allowed-ips/{id}":   {"allowlist.update"},
	"DELETE /api/v1/settings/allowed-ips/{id}":  {"allowlist.remove"},
	"PUT /api/v1/settings/allowed-ips/mode":     {"allowlist.mode"},
	"POST /api/v1/settings/allowed-ips/confirm": {"allowlist.confirm"},
	"PUT /api/v1/users/{id}/site-roles":         {"user.site_roles.update"},
	// Facility (v0.4.1): before/after of the changed fields; a rack update that changes the name is rack.rename.
	"POST /api/v1/sites":                  {"site.create"},
	"PATCH /api/v1/sites/{id}":            {"site.update"},
	"POST /api/v1/sites/{id}/archive":     {"site.archive"},
	"POST /api/v1/buildings":              {"building.create"},
	"PATCH /api/v1/buildings/{id}":        {"building.update"},
	"POST /api/v1/buildings/{id}/archive": {"building.archive"},
	"POST /api/v1/floors":                 {"floor.create"},
	"PATCH /api/v1/floors/{id}":           {"floor.update"},
	"POST /api/v1/floors/{id}/archive":    {"floor.archive"},
	"POST /api/v1/rooms":                  {"room.create"},
	"PATCH /api/v1/rooms/{id}":            {"room.update"},
	"POST /api/v1/rooms/{id}/archive":     {"room.archive"},
	"POST /api/v1/racks":                  {"rack.create"},
	"PATCH /api/v1/racks/{id}":            {"rack.update", "rack.rename"},
	"POST /api/v1/racks/{id}/archive":     {"rack.archive"},
}

// UnauditedRoutes are unsafe routes deliberately not audited, each with the reason (reviewed like AuditedRoutes).
var UnauditedRoutes = map[string]string{
	"POST /api/v1/auth/activity": "idle heartbeat about once a minute (D-069): changes only the session's idle deadline",
}

// Mount returns the /api/v1 mount function. internal selects the loopback listener variant.
func Mount(m Modules, internal bool) func(r chi.Router) {
	return func(r chi.Router) {
		// Allowed IP (v0.3.6): the public listener enforces the allowlist decision (second layer behind Caddy's
		// forward_auth); the internal listener (loopback, ops scripts) is exempt and serves Caddy's check endpoint.
		if !internal && m.Allowlist != nil {
			r.Use(m.Allowlist.Middleware)
		}
		// Public group — keep in sync with PublicRoutes / InternalPublicRoutes.
		r.Group(func(r chi.Router) {
			m.System.PublicRoutes(r, m.Auth.Identify)
			m.Auth.PublicRoutes(r)
			if internal {
				m.System.OpsRoutes(r)
				m.Allowlist.CheckRoutes(r)
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
			m.Settings.Routes(r, m.Auth.Require)
			m.Allowlist.Routes(r, m.Auth.Require)
			if m.Facility != nil {
				m.Facility.Routes(r, m.Auth.Require)
			}
		})
	}
}
