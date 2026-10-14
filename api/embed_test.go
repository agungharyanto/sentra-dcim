package apispec

import (
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

var methods = []string{"get", "put", "post", "delete", "patch", "head", "options"}

// operations returns "METHOD path" → operation of the embedded spec (path-level keys such as parameters skipped).
func operations(t *testing.T) map[string]map[string]any {
	t.Helper()
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(OpenAPI, &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]map[string]any{}
	for path, item := range doc.Paths {
		for method, v := range item {
			if op, ok := v.(map[string]any); ok && slices.Contains(methods, method) {
				out[method+" "+path] = op
			}
		}
	}
	return out
}

// Every operation declares who may call it at the public edge (x-sentra-access); the Help Center API viewer shows
// it. "public" must stay exactly the edge allowlist in deploy/caddy/Caddyfile (@public).
func TestEveryOperationDeclaresAccess(t *testing.T) {
	var public []string
	for key, op := range operations(t) {
		access, _ := op["x-sentra-access"].(string)
		if !slices.Contains([]string{"public", "internal", "login"}, access) {
			t.Errorf("%s: x-sentra-access = %q, want public | internal | login", key, access)
		}
		if access == "public" {
			public = append(public, key[len("get "):])
		}
	}
	slices.Sort(public)
	if want := []string{"/api/v1/health", "/api/v1/version"}; !slices.Equal(public, want) {
		t.Errorf("public operations = %v, want %v (Caddy @public allowlist)", public, want)
	}
}

// x-sentra-permission names a permission defined in code and only appears on session ("login") operations. That
// the router requires exactly the documented permission is checked by test/integration TestPermissionDefaultDeny.
func TestPermissionsAreKnown(t *testing.T) {
	n := 0
	for key, op := range operations(t) {
		perm, ok := op["x-sentra-permission"].(string)
		if !ok {
			continue
		}
		n++
		if !slices.Contains(rbac.All, rbac.Permission(perm)) {
			t.Errorf("%s: unknown x-sentra-permission %q", key, perm)
		}
		if op["x-sentra-access"] != "login" {
			t.Errorf("%s: x-sentra-permission on a non-login operation", key)
		}
	}
	if n == 0 {
		t.Fatal("no operation declares x-sentra-permission")
	}
}
