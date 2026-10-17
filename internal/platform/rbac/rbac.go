// Package rbac defines permissions and the permission sets of the system roles (ADR-003, D-050, D-058).
// Permissions live in code (reviewed, versioned, tested); roles and their assignment to users live in the database
// (table roles / user_roles, migration 000004). A permission is granted when any role of the user grants it.
package rbac

import (
	"slices"
)

// Permission is a capability checked by route middleware ("<module>.<action>").
type Permission string

// Permissions. Add a permission here together with the routes that require it and the roles that grant it.
const (
	UsersRead         Permission = "users.read"          // list/view accounts
	UsersManage       Permission = "users.manage"        // create, edit, disable/enable, reset password, assign roles
	RolesRead         Permission = "roles.read"          // list roles and their permissions
	SystemVersionRead Permission = "system.version.read" // build details of /api/v1/version (commit, Go, server clock)
	UIDesignSystem    Permission = "ui.design_system"    // developer showcase /design-system (UI only, ISS-029)
	AuditRead         Permission = "audit.read"          // audit log viewer and hash-chain integrity status/verification
	AuditExport       Permission = "audit.export"        // CSV export of the audit log (itself audited)
	HelpAdmin         Permission = "help.admin"          // Help Center pages for administrators (audience admin, ISS-032)
	SettingsManage    Permission = "settings.manage"     // read and change system settings (/api/v1/settings/*, v0.3.5)
	FacilityRead      Permission = "facility.read"       // sites, buildings, floors, rooms and the facility tree (v0.4.1)
	FacilityManage    Permission = "facility.manage"     // create, edit and archive sites, buildings, floors and rooms
	RackRead          Permission = "rack.read"           // racks (v0.4.1)
	RackManage        Permission = "rack.manage"         // create, edit, rename, move and archive racks
)

// All lists every permission (stable order, used by docs and tests).
var All = []Permission{UsersRead, UsersManage, RolesRead, SystemVersionRead, UIDesignSystem, AuditRead, AuditExport, HelpAdmin,
	SettingsManage, FacilityRead, FacilityManage, RackRead, RackManage}

// SiteScoped are the permissions a role assignment limited to one site (user_roles.site_id, D-084) can grant: they
// guard data that belongs to a site. Every other permission is granted by global assignments only, so a role given
// for one site never reaches users, settings or the audit log.
var SiteScoped = []Permission{FacilityRead, FacilityManage, RackRead, RackManage}

// IsSiteScoped reports whether p can be granted per site.
func IsSiteScoped(p Permission) bool { return slices.Contains(SiteScoped, p) }

// facilityRead is what every operational role may see of the facility (v0.4.1, D-084).
var facilityRead = []Permission{FacilityRead, RackRead}

// System role keys (seeded by migration 000004; labels are translated by the UI: users:roles.<key>).
const (
	RoleAdmin      = "admin"
	RoleNOC        = "noc"
	RoleSecurity   = "security"
	RoleTechnician = "technician"
	RoleViewer     = "viewer"
	RoleAuditor    = "auditor"
)

// SystemRoleKeys lists the system roles in display order.
var SystemRoleKeys = []string{RoleAdmin, RoleNOC, RoleSecurity, RoleTechnician, RoleViewer, RoleAuditor}

// systemRoles maps each system role to its permissions. Every role reads the facility (v0.4.1); only Admin changes
// it. Further module permissions arrive with their modules.
var systemRoles = map[string][]Permission{
	RoleAdmin:      All,
	RoleNOC:        facilityRead,
	RoleSecurity:   facilityRead,
	RoleTechnician: facilityRead,
	RoleViewer:     facilityRead,
	RoleAuditor:    append([]Permission{UsersRead, RolesRead, AuditRead, AuditExport}, facilityRead...),
}

// SiteAssignable reports whether role may be assigned for a single site. Admin is global by definition (it manages
// users, settings and the audit log, which have no site).
func SiteAssignable(role string) bool { return IsSystemRole(role) && role != RoleAdmin }

// RolePermissions returns the permissions of a role (nil for an unknown role).
func RolePermissions(role string) []Permission {
	p, ok := systemRoles[role]
	if !ok {
		return nil
	}
	return slices.Clone(p)
}

// IsSystemRole reports whether key is a system role.
func IsSystemRole(key string) bool { _, ok := systemRoles[key]; return ok }

// Resolve returns the sorted, de-duplicated permissions granted by roles. Unknown roles grant nothing.
func Resolve(roles []string) []Permission {
	var out []Permission
	for _, r := range roles {
		out = append(out, systemRoles[r]...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// ResolveSite returns the permissions a role assigned for one site grants: its site-scoped permissions only.
func ResolveSite(role string) []Permission {
	var out []Permission
	for _, p := range systemRoles[role] {
		if IsSiteScoped(p) {
			out = append(out, p)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Strings converts permissions to strings (JSON responses).
func Strings(ps []Permission) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = string(p)
	}
	return out
}
