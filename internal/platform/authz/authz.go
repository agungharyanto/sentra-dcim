// Package authz turns the caller's permissions into a data scope (D-084, v0.4.1). A permission guarding site data
// (rbac.SiteScoped) may be granted globally (role assignment without a site) or for single sites; repositories of
// site data take a Scope and filter every query by it, so a caller never sees, changes or learns about rows of a site
// outside it (out-of-scope lookups by ID answer 404, like a missing row).
package authz

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

// Scope is the set of sites a permission reaches: every site (All) or the listed ones. The zero Scope reaches
// nothing.
type Scope struct {
	All     bool
	SiteIDs []uuid.UUID
}

// Global is the scope of a global grant (and of system/CLI callers).
var Global = Scope{All: true}

// Allows reports whether site is inside the scope.
func (s Scope) Allows(site uuid.UUID) bool { return s.All || slices.Contains(s.SiteIDs, site) }

// Empty reports whether the scope reaches no site at all.
func (s Scope) Empty() bool { return !s.All && len(s.SiteIDs) == 0 }

// IDs returns the site IDs for SQL filters (`$all OR site_id = ANY($ids)`); never nil, so the array parameter is
// always a valid (possibly empty) uuid[].
func (s Scope) IDs() []uuid.UUID {
	if s.SiteIDs == nil {
		return []uuid.UUID{}
	}
	return s.SiteIDs
}

// For returns the scope of perm for p: global when any global assignment grants it, else the sites whose assignments
// grant it (sorted). A permission that is not site-scoped is either global or empty.
func For(p principal.Principal, perm rbac.Permission) Scope {
	if slices.Contains(p.Permissions, perm) {
		return Global
	}
	if !rbac.IsSiteScoped(perm) {
		return Scope{}
	}
	var ids []uuid.UUID
	for site, ps := range p.Sites {
		if slices.Contains(ps, perm) {
			ids = append(ids, site)
		}
	}
	slices.SortFunc(ids, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	return Scope{SiteIDs: ids}
}

// FromContext returns the scope of perm for the request principal; without a principal the scope is empty.
func FromContext(ctx context.Context, perm rbac.Permission) Scope {
	p, ok := principal.From(ctx)
	if !ok {
		return Scope{}
	}
	return For(p, perm)
}
