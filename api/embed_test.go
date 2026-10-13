package apispec

import (
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"
)

// Every operation declares who may call it at the public edge (x-sentra-access); the Help Center API viewer shows
// it. "public" must stay exactly the edge allowlist in deploy/caddy/Caddyfile (@public).
func TestEveryOperationDeclaresAccess(t *testing.T) {
	var doc struct {
		Paths map[string]map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(OpenAPI, &doc); err != nil {
		t.Fatal(err)
	}
	var public []string
	for path, item := range doc.Paths {
		for method, op := range item {
			access, _ := op["x-sentra-access"].(string)
			if !slices.Contains([]string{"public", "internal", "login"}, access) {
				t.Errorf("%s %s: x-sentra-access = %q, want public | internal | login", method, path, access)
			}
			if access == "public" {
				public = append(public, path)
			}
		}
	}
	slices.Sort(public)
	if want := []string{"/api/v1/health", "/api/v1/version"}; !slices.Equal(public, want) {
		t.Errorf("public operations = %v, want %v (Caddy @public allowlist)", public, want)
	}
}
