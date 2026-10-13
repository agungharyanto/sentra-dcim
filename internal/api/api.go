// Package api assembles the /api/v1 routes of both listeners. Everything is default-deny: a route is reachable
// without a session only when it is registered in the public group below (see PublicRoutes and api_test.go).
package api

import (
	"github.com/go-chi/chi/v5"

	authhandler "github.com/agungharyanto/sentra-dcim/internal/auth/handler"
	helphandler "github.com/agungharyanto/sentra-dcim/internal/help/handler"
	systemhandler "github.com/agungharyanto/sentra-dcim/internal/system/handler"
	uihandler "github.com/agungharyanto/sentra-dcim/internal/ui/handler"
)

// Modules are the HTTP modules mounted under /api/v1.
type Modules struct {
	System *systemhandler.Handler
	UI     *uihandler.Handler
	Help   *helphandler.Handler
	Auth   *authhandler.Handler
}

// PublicRoutes is the allowlist of "METHOD /path" reachable without a session on the public listener.
var PublicRoutes = []string{
	"GET /api/v1/health",
	"GET /api/v1/version",
	"POST /api/v1/auth/login",
}

// InternalPublicRoutes extends PublicRoutes on the internal (loopback-only) listener for ops scripts.
var InternalPublicRoutes = append(append([]string{}, PublicRoutes...), "GET /api/v1/ready")

// Mount returns the /api/v1 mount function. internal selects the loopback listener variant.
func Mount(m Modules, internal bool) func(r chi.Router) {
	return func(r chi.Router) {
		// Public group — keep in sync with PublicRoutes / InternalPublicRoutes.
		r.Group(func(r chi.Router) {
			m.System.PublicRoutes(r)
			m.Auth.PublicRoutes(r)
			if internal {
				m.System.OpsRoutes(r)
			}
		})
		// Everything else requires a session, passes CSRF for unsafe methods and is blocked while the user must
		// change the password.
		r.Group(func(r chi.Router) {
			r.Use(m.Auth.Authenticate, m.Auth.CSRF, authhandler.PasswordChangeGuard)
			if !internal {
				m.System.OpsRoutes(r)
			}
			m.Auth.Routes(r)
			m.UI.Routes(r)
			m.Help.Routes(r)
		})
	}
}
