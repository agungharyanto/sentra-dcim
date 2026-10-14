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
	if got := Resolve([]string{"nope", RoleViewer}); len(got) != 0 {
		t.Errorf("unknown + viewer = %v", got)
	}
	got := Resolve([]string{RoleAuditor, RoleAuditor, RoleViewer})
	if !slices.Equal(got, []Permission{AuditExport, AuditRead, RolesRead, UsersRead}) {
		t.Errorf("auditor = %v", got)
	}
	// Least privilege: only Admin may manage users or read build details.
	for _, r := range SystemRoleKeys {
		ps := Resolve([]string{r})
		if r != RoleAdmin && (slices.Contains(ps, UsersManage) || slices.Contains(ps, SystemVersionRead) || slices.Contains(ps, UIDesignSystem) ||
			slices.Contains(ps, HelpAdmin)) {
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

func sorted(ps []Permission) []Permission { c := slices.Clone(ps); slices.Sort(c); return c }
