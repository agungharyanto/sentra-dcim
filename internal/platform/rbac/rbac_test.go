package rbac

import (
	"regexp"
	"slices"
	"testing"
)

func TestAdminHasEveryPermission(t *testing.T) {
	if got := Resolve([]string{RoleAdmin}); !slices.Equal(got, sorted(All)) {
		t.Fatalf("admin = %v, want all %v", got, All)
	}
}

func TestEverySystemRoleDefined(t *testing.T) {
	if len(systemRoles) != len(SystemRoleKeys) {
		t.Fatalf("systemRoles has %d roles, SystemRoleKeys %d", len(systemRoles), len(SystemRoleKeys))
	}
	keyRE := regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`) // same CHECK as roles.key (migration 000004)
	for _, k := range SystemRoleKeys {
		if !IsSystemRole(k) || !keyRE.MatchString(k) {
			t.Errorf("role %q not defined or invalid key", k)
		}
	}
}

func TestPermissionsWellFormedAndKnown(t *testing.T) {
	re := regexp.MustCompile(`^[a-z]+(\.[a-z_]+)+$`)
	for _, p := range All {
		if !re.MatchString(string(p)) {
			t.Errorf("permission %q malformed", p)
		}
	}
	for role, ps := range systemRoles {
		for _, p := range ps {
			if !slices.Contains(All, p) {
				t.Errorf("role %s grants unknown permission %q", role, p)
			}
		}
	}
}

func TestResolve(t *testing.T) {
	if got := Resolve(nil); len(got) != 0 {
		t.Errorf("no roles = %v", got)
	}
	if got := Resolve([]string{"nope", RoleViewer}); !slices.Equal(got, []Permission{FacilityRead, RackRead}) {
		t.Errorf("unknown + viewer = %v", got)
	}
	got := Resolve([]string{RoleAuditor, RoleAuditor, RoleViewer})
	if !slices.Equal(got, []Permission{AuditExport, AuditRead, FacilityRead, RackRead, RolesRead, UsersRead}) {
		t.Errorf("auditor = %v", got)
	}
	// Least privilege: only Admin may manage users or read build details.
	for _, r := range SystemRoleKeys {
		ps := Resolve([]string{r})
		if r != RoleAdmin && (slices.Contains(ps, UsersManage) || slices.Contains(ps, SystemVersionRead) || slices.Contains(ps, UIDesignSystem) ||
			slices.Contains(ps, HelpAdmin) || slices.Contains(ps, FacilityManage) || slices.Contains(ps, RackManage)) {
			t.Errorf("role %s must not have %v", r, ps)
		}
	}
	// RolePermissions returns a copy.
	p := RolePermissions(RoleAdmin)
	p[0] = "x.y"
	if RolePermissions(RoleAdmin)[0] == "x.y" {
		t.Error("RolePermissions leaks internal slice")
	}
}

// v0.4.1 (D-084): every role reads the facility; a site assignment grants only site-scoped permissions; Admin is
// never assigned per site.
func TestFacilityAndSiteScope(t *testing.T) {
	for _, r := range SystemRoleKeys {
		ps := Resolve([]string{r})
		if !slices.Contains(ps, FacilityRead) || !slices.Contains(ps, RackRead) {
			t.Errorf("role %s cannot read the facility: %v", r, ps)
		}
		for _, p := range ResolveSite(r) {
			if !IsSiteScoped(p) {
				t.Errorf("site assignment of %s grants global permission %s", r, p)
			}
		}
	}
	if got := ResolveSite(RoleAuditor); !slices.Equal(got, []Permission{FacilityRead, RackRead}) {
		t.Errorf("auditor per site = %v", got)
	}
	if SiteAssignable(RoleAdmin) || SiteAssignable("nope") || !SiteAssignable(RoleTechnician) {
		t.Error("SiteAssignable: admin/unknown must be global only")
	}
	for _, p := range SiteScoped {
		if !slices.Contains(All, p) {
			t.Errorf("site-scoped %s not in All", p)
		}
	}
	if IsSiteScoped(UsersManage) || IsSiteScoped(SettingsManage) || IsSiteScoped(AuditRead) {
		t.Error("global permissions must not be site-scoped")
	}
}

func sorted(ps []Permission) []Permission { c := slices.Clone(ps); slices.Sort(c); return c }
