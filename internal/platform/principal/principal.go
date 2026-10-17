// Package principal carries the authenticated caller through the request context (ADR-003). The type field makes
// room for machine tokens (v0.6) next to users.
package principal

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

// Types of principals.
const (
	TypeUser = "user"
)

// Principal is who is making the request.
type Principal struct {
	Type      string    `json:"type"`
	ID        uuid.UUID `json:"id"`
	Username  string    `json:"username"`
	SessionID uuid.UUID `json:"-"` // session that authenticated the request (users only)
	// Roles and the permissions they grant (rbac.Resolve), loaded with the session on every request so role
	// changes apply immediately. Roles and Permissions are the global assignments (user_roles.site_id NULL).
	Roles       []string          `json:"-"`
	Permissions []rbac.Permission `json:"-"`
	// Sites holds the site-scoped permissions of role assignments limited to one site (D-084, v0.4.1):
	// site ID → rbac.ResolveSite of the roles assigned there. Global permissions apply to every site as well.
	Sites map[uuid.UUID][]rbac.Permission `json:"-"`
}

// Can reports whether the principal holds permission p globally or, for a site-scoped permission, for at least one
// site. Route middleware uses it; the data layer narrows the rows with authz.Scope.
func (p Principal) Can(perm rbac.Permission) bool {
	if slices.Contains(p.Permissions, perm) {
		return true
	}
	if !rbac.IsSiteScoped(perm) {
		return false
	}
	for _, ps := range p.Sites {
		if slices.Contains(ps, perm) {
			return true
		}
	}
	return false
}

// Effective returns every permission the principal holds anywhere (global ∪ any site), sorted — what the UI uses
// to show menus and actions.
func (p Principal) Effective() []rbac.Permission {
	out := slices.Clone(p.Permissions)
	for _, ps := range p.Sites {
		out = append(out, ps...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

type ctxKey struct{}

// With returns ctx carrying p.
func With(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxKey{}, p)
}

// From returns the principal of ctx, if any.
func From(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(ctxKey{}).(Principal)
	return p, ok
}
