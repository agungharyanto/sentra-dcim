//go:build integration

package integration

import (
	"fmt"
	"net/http"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	authhandler "github.com/agungharyanto/sentra-dcim/internal/auth/handler"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

// TestPermissionMatrix (v0.3.9 sprint close): every system role signs in and calls every route that OpenAPI
// documents with x-sentra-permission. A role holding the permission must get past the permission check (any answer
// but 403 forbidden); a role without it must get exactly 403 forbidden. With SENTRA_PERM_MATRIX_OUT set, the matrix
// is written there as Markdown (docs/releases/v0.3.9.md quotes it).
func TestPermissionMatrix(t *testing.T) {
	e := newAuthEnv(t, nil)
	documented := documentedPermissions(t)
	routes := make([]string, 0, len(documented))
	for r := range documented {
		routes = append(routes, r)
	}
	sort.Slice(routes, func(i, j int) bool {
		pi, pj := strings.SplitN(routes[i], " ", 2)[1], strings.SplitN(routes[j], " ", 2)[1]
		if pi != pj {
			return pi < pj
		}
		return routes[i] < routes[j]
	})
	if len(routes) < 20 {
		t.Fatalf("only %d permission-guarded routes documented", len(routes))
	}
	fake := "00000000-0000-4000-8000-000000000000"
	result := map[string]map[string]string{} // route → role → "✓" / "—"
	for _, role := range rbac.SystemRoleKeys {
		c := e.signIn("matrix."+role, role)
		perms := rbac.Resolve([]string{role})
		for _, rt := range routes {
			method, path, _ := strings.Cut(rt, " ")
			path = strings.NewReplacer("{id}", fake, "{group}", "regional").Replace(path)
			// Harmless bodies: empty objects are refused by validation or hit a missing id — never a real change.
			var body any
			if method != http.MethodGet {
				body = map[string]any{}
			}
			if strings.HasSuffix(rt, "/export.csv") {
				path += "?since=2099-01-01T00:00:00Z"
			}
			r := c.do(method, path, body)
			allowed := slices.Contains(perms, rbac.Permission(documented[rt]))
			forbidden := r.code == http.StatusForbidden && r.errCode() == authhandler.CodeForbidden
			if allowed == forbidden {
				t.Errorf("%-8s %s: %s → %d %q, want allowed=%t (permission %s)", role, method, path, r.code, r.errCode(), allowed, documented[rt])
			}
			if result[rt] == nil {
				result[rt] = map[string]string{}
			}
			mark := "—"
			if !forbidden {
				mark = "✓"
			}
			result[rt][role] = mark
		}
	}
	if out := os.Getenv("SENTRA_PERM_MATRIX_OUT"); out != "" {
		var b strings.Builder
		b.WriteString("| Endpoint | Izin |")
		for _, role := range rbac.SystemRoleKeys {
			b.WriteString(" " + role + " |")
		}
		b.WriteString("\n|---|---|" + strings.Repeat("---|", len(rbac.SystemRoleKeys)) + "\n")
		for _, rt := range routes {
			fmt.Fprintf(&b, "| `%s` | `%s` |", rt, documented[rt])
			for _, role := range rbac.SystemRoleKeys {
				b.WriteString(" " + result[rt][role] + " |")
			}
			b.WriteString("\n")
		}
		if err := os.WriteFile(out, []byte(b.String()), 0o644); err != nil { //nolint:gosec // test artifact
			t.Fatal(err)
		}
	}
	t.Logf("%d routes × %d roles checked", len(routes), len(rbac.SystemRoleKeys))
}
