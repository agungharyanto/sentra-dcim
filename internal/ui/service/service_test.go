package service

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

var update = flag.Bool("update", false, "rewrite web/src/test/fixtures/ui-registry.json from the registry")

// fixturePath is shared with the UI tests: web/src/app/routes.test.ts fails when an available menu has no page.
var fixturePath = filepath.Join("..", "..", "..", "web", "src", "test", "fixtures", "ui-registry.json")

func TestRegistryIsConsistent(t *testing.T) {
	gs, menus := Registry()
	groupKeys := map[string]bool{}
	for _, g := range gs {
		if g.Key == "" || groupKeys[g.Key] {
			t.Fatalf("empty or duplicate group %q", g.Key)
		}
		groupKeys[g.Key] = true
	}
	keys, paths := map[string]bool{}, map[string]bool{}
	used := map[string]bool{}
	for _, m := range menus {
		switch {
		case m.Key == "" || keys[m.Key]:
			t.Fatalf("empty or duplicate menu key %q", m.Key)
		case !strings.HasPrefix(m.Path, "/") || m.Path == "/" || paths[m.Path]:
			t.Fatalf("menu %q: path %q must start with /, not be / and be unique", m.Key, m.Path)
		case !groupKeys[m.Group]:
			t.Fatalf("menu %q: unknown group %q", m.Key, m.Group)
		case !slices.Contains([]string{StatusAvailable, StatusComingSoon, StatusNotConnected}, m.Status):
			t.Fatalf("menu %q: invalid status %q", m.Key, m.Status)
		case m.Icon == "":
			t.Fatalf("menu %q: missing icon", m.Key)
		case m.Permission != "" && !slices.Contains(rbac.All, rbac.Permission(m.Permission)):
			t.Fatalf("menu %q: unknown permission %q", m.Key, m.Permission)
		}
		keys[m.Key], paths[m.Path], used[m.Group] = true, true, true
	}
	for g := range groupKeys {
		if !used[g] {
			t.Fatalf("group %q has no menus", g)
		}
	}
}

func TestRegistryGrouping(t *testing.T) {
	_, menus := Registry()
	want := map[string][]string{
		GroupMonitoring:     {"dashboard", "alarms", "power", "environment"},
		GroupInfrastructure: {"facility", "assets", "tenants"},
		GroupOperations:     {"access", "visitors", "maintenance", "analytics"},
		GroupSystem:         {"help", "users", "settings"},
	}
	got := map[string][]string{}
	for _, m := range menus {
		got[m.Group] = append(got[m.Group], m.Key)
	}
	for g, keys := range want {
		if !slices.Equal(got[g], keys) {
			t.Fatalf("group %s = %v, want %v", g, got[g], keys)
		}
	}
}

// The UI checks the registry it will receive against its page map using this fixture; keep it in sync.
func TestRegistryFixtureUpToDate(t *testing.T) {
	gs, menus := Registry()
	want, err := json.MarshalIndent(map[string]any{"menu_groups": gs, "menus": menus}, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')
	if *update {
		if err := os.WriteFile(fixturePath, want, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("%v (run: go test ./internal/ui/service -update)", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s is stale; run: go test ./internal/ui/service -update", fixturePath)
	}
}
