//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.yaml.in/yaml/v3"

	apispec "github.com/agungharyanto/sentra-dcim/api"

	"github.com/agungharyanto/sentra-dcim/internal/api"
	auditservice "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	authhandler "github.com/agungharyanto/sentra-dcim/internal/auth/handler"
	authservice "github.com/agungharyanto/sentra-dcim/internal/auth/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	"github.com/agungharyanto/sentra-dcim/internal/platform/migrator"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
	usersservice "github.com/agungharyanto/sentra-dcim/internal/users/service"
)

var tempPwRE = regexp.MustCompile(`^[a-km-zA-HJ-NP-Z2-9]{4}(-[a-km-zA-HJ-NP-Z2-9]{4}){3}$`)

// setRoles replaces the roles of username directly in the database.
func (e *authEnv) setRoles(username string, roles ...string) {
	e.t.Helper()
	e.exec(`DELETE FROM user_roles WHERE user_id = (SELECT id FROM users WHERE username = $1)`, username)
	for _, r := range roles {
		e.exec(`INSERT INTO user_roles (user_id, role_id) SELECT u.id, r.id FROM users u, roles r WHERE u.username = $1 AND r.key = $2`,
			username, r)
	}
}

// signIn creates a ready account with roles and returns a signed-in client.
func (e *authEnv) signIn(username string, roles ...string) *client {
	e.t.Helper()
	e.createUser(username, true)
	e.setRoles(username, roles...)
	c := e.client()
	want(e.t, "login "+username, c.login(username, pwInitial), http.StatusOK, "")
	return c
}

func userIDOf(t *testing.T, r resp) string {
	t.Helper()
	u, ok := r.body["user"].(map[string]any)
	if !ok {
		u = r.body
	}
	id, _ := u["id"].(string)
	if id == "" {
		t.Fatalf("no user id in %v", r.body)
	}
	return id
}

func strs(v any) []string {
	var out []string
	for _, x := range v.([]any) {
		out = append(out, x.(string))
	}
	return out
}

// TestMigrationSeedsSystemRoles: the roles table holds exactly the system roles defined in code.
func TestMigrationSeedsSystemRoles(t *testing.T) {
	e := newAuthEnv(t, nil)
	rows, err := e.pool.Query(context.Background(), "SELECT key FROM roles WHERE system ORDER BY key")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for rows.Next() {
		var k string
		_ = rows.Scan(&k)
		keys = append(keys, k)
	}
	want := slices.Clone(rbac.SystemRoleKeys)
	slices.Sort(want)
	if !slices.Equal(keys, want) {
		t.Fatalf("roles in DB %v, code %v", keys, want)
	}
}

// TestMigration000004MakesExistingAccountsAdmin: accounts of v0.3.1 (created only via create-admin) become Admin,
// and 000004 down/up is clean.
func TestMigration000004MakesExistingAccountsAdmin(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	dsn := tempDB(t)
	m, err := migrator.New(dsn, logging.Discard())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()
	if err := m.Goto(3); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO users (username, display_name, password_hash) VALUES ('old.admin', 'Old', '$argon2id$x')`); err != nil {
		t.Fatal(err)
	}
	for i, step := range []func() error{m.Up, func() error { return m.Down(1) }, m.Up} {
		if err := step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	var role string
	if err := pool.QueryRow(ctx, `SELECT r.key FROM user_roles ur JOIN roles r ON r.id = ur.role_id
		JOIN users u ON u.id = ur.user_id WHERE u.username = 'old.admin'`).Scan(&role); err != nil || role != "admin" {
		t.Fatalf("old.admin role = %q (%v), want admin", role, err)
	}
}

// TestPermissionDefaultDeny signs in without any role and requests every authenticated route: everything outside
// api.SelfServiceRoutes must answer 403 "forbidden" (a route that forgets Auth.Require fails here), and every
// self-service route must not.
func TestPermissionDefaultDeny(t *testing.T) {
	e := newAuthEnv(t, nil)
	c := e.signIn("no.roles")
	var all []string
	err := chi.Walk(e.srv.Config.Handler.(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		all = append(all, method+" "+strings.ReplaceAll(route, "/*/", "/"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, rt := range all {
		if slices.Contains(api.PublicRoutes, rt) || rt == "POST /api/v1/auth/logout" {
			continue // public, or ends the session used by this test
		}
		method, path, _ := strings.Cut(rt, " ")
		path = strings.NewReplacer("{id}", "00000000-0000-4000-8000-000000000000", "*", "dashboard").Replace(path)
		r := c.do(method, path, map[string]any{})
		self := slices.Contains(api.SelfServiceRoutes, rt)
		switch {
		case self && r.code == http.StatusForbidden:
			t.Errorf("self-service %s: 403 for a signed-in user", rt)
		case !self && (r.code != http.StatusForbidden || r.errCode() != authhandler.CodeForbidden):
			t.Errorf("%s without permission: got %d %q, want 403 forbidden", rt, r.code, r.errCode())
		}
		checked++
	}
	if checked < 15 {
		t.Fatalf("only %d routes checked: %v", checked, all)
	}
	// The permission each route really requires (recorded by authz.denied) must be the one documented in
	// api/openapi.yaml (x-sentra-permission), and every documented permission must be enforced.
	enforced := map[string]string{}
	rows, err := e.pool.Query(context.Background(),
		`SELECT target_id, metadata->>'permission' FROM audit_events WHERE action = $1`, authservice.ActionAuthzDenied)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var route, perm string
		_ = rows.Scan(&route, &perm)
		enforced[route] = perm
	}
	if len(enforced) == 0 {
		t.Fatal("no authz.denied audit events")
	}
	documented := documentedPermissions(t)
	for route, perm := range enforced {
		if documented[route] != perm {
			t.Errorf("%s requires %q, OpenAPI x-sentra-permission = %q", route, perm, documented[route])
		}
	}
	for route, perm := range documented {
		if enforced[route] != perm {
			t.Errorf("%s documents %q but the router requires %q", route, perm, enforced[route])
		}
	}
}

// documentedPermissions maps "METHOD /path" to x-sentra-permission of api/openapi.yaml.
func documentedPermissions(t *testing.T) map[string]string {
	t.Helper()
	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(apispec.OpenAPI, &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for path, item := range doc.Paths {
		for method, v := range item {
			if op, ok := v.(map[string]any); ok {
				if p, ok := op["x-sentra-permission"].(string); ok {
					out[strings.ToUpper(method)+" "+path] = p
				}
			}
		}
	}
	return out
}

// TestUsersAdminFlow: create → first login with the temporary password → roles apply immediately → disable
// revokes sessions → enable → reset password.
func TestUsersAdminFlow(t *testing.T) {
	e := newAuthEnv(t, nil)
	admin := e.signIn("admin.one", rbac.RoleAdmin)

	r := admin.post("/api/v1/users", map[string]any{"username": "Teknisi.01", "display_name": " Teknisi Satu ",
		"email": "teknisi@example.go.id", "roles": []string{"technician"}})
	want(t, "create", r, http.StatusCreated, "")
	if r.header.Get("Cache-Control") != "no-store" {
		t.Errorf("create Cache-Control = %q", r.header.Get("Cache-Control"))
	}
	temp, _ := r.body["temporary_password"].(string)
	if !tempPwRE.MatchString(temp) {
		t.Fatalf("temporary password format: %q", strings.Repeat("*", len(temp)))
	}
	id := userIDOf(t, r)
	u := r.body["user"].(map[string]any)
	if u["display_name"] != "Teknisi Satu" || u["must_change_password"] != true || !slices.Equal(strs(u["roles"]), []string{"technician"}) {
		t.Fatalf("created user %v", u)
	}

	tech := e.client()
	want(t, "login with temporary password", tech.login("teknisi.01", temp), http.StatusOK, "")
	want(t, "must change first", tech.get("/api/v1/ui/bootstrap"), http.StatusForbidden, "password_change_required")
	want(t, "change", tech.post("/api/v1/auth/password", map[string]string{"current_password": temp, "new_password": pwNew}),
		http.StatusNoContent, "")
	want(t, "technician cannot list users", tech.get("/api/v1/users"), http.StatusForbidden, "forbidden")

	// Roles apply on the next request, without a new login.
	want(t, "grant auditor", admin.put("/api/v1/users/"+id+"/roles", map[string]any{"roles": []string{"technician", "auditor"}}),
		http.StatusOK, "")
	want(t, "auditor lists users", tech.get("/api/v1/users"), http.StatusOK, "")
	want(t, "auditor reads roles", tech.get("/api/v1/roles"), http.StatusOK, "")
	want(t, "auditor cannot create", tech.post("/api/v1/users", map[string]any{"username": "x.y.z", "display_name": "x",
		"roles": []string{"viewer"}}), http.StatusForbidden, "forbidden")
	me := tech.get("/api/v1/auth/me")
	if p := strs(me.body["user"].(map[string]any)["permissions"]); !slices.Equal(p, []string{"audit.export", "audit.read", "roles.read", "users.read"}) {
		t.Fatalf("auditor permissions in /me = %v", p)
	}

	// List, search (literal: % is not a wildcard), filters.
	l := admin.get("/api/v1/users?q=teknisi&status=active&role=auditor")
	if l.body["total"] != float64(1) {
		t.Fatalf("search total = %v", l.body["total"])
	}
	if l := admin.get("/api/v1/users?q=%25"); l.body["total"] != float64(0) {
		t.Fatalf("q=%% matched %v", l.body["total"])
	}
	want(t, "bad status filter", admin.get("/api/v1/users?status=x"), http.StatusUnprocessableEntity, "validation_failed")

	// Profile.
	p := admin.patch("/api/v1/users/"+id, map[string]any{"display_name": "Teknisi Utama", "email": ""})
	want(t, "patch", p, http.StatusOK, "")
	if p.body["display_name"] != "Teknisi Utama" || p.body["email"] != nil {
		t.Fatalf("patched %v", p.body)
	}
	want(t, "username is immutable", admin.patch("/api/v1/users/"+id, map[string]any{"username": "x"}), http.StatusBadRequest, "invalid_request")

	// Disable revokes the user's sessions immediately; login is refused generically.
	d := admin.post("/api/v1/users/"+id+"/disable", nil)
	want(t, "disable", d, http.StatusOK, "")
	if d.body["status"] != "disabled" {
		t.Fatalf("status %v", d.body["status"])
	}
	want(t, "disabled session", tech.get("/api/v1/auth/me"), http.StatusUnauthorized, "unauthenticated")
	want(t, "disabled login", e.client().login("teknisi.01", pwNew), http.StatusUnauthorized, "invalid_credentials")
	want(t, "enable", admin.post("/api/v1/users/"+id+"/enable", nil), http.StatusOK, "")
	tech2 := e.client()
	want(t, "login after enable", tech2.login("teknisi.01", pwNew), http.StatusOK, "")

	// Reset: new temporary password, sessions revoked, old password invalid, must change again.
	rs := admin.post("/api/v1/users/"+id+"/password-reset", nil)
	want(t, "reset", rs, http.StatusOK, "")
	temp2, _ := rs.body["temporary_password"].(string)
	if !tempPwRE.MatchString(temp2) || temp2 == temp || rs.header.Get("Cache-Control") != "no-store" {
		t.Fatal("reset: bad temporary password or cacheable response")
	}
	want(t, "session revoked by reset", tech2.get("/api/v1/auth/me"), http.StatusUnauthorized, "unauthenticated")
	want(t, "old password", e.client().login("teknisi.01", pwNew), http.StatusUnauthorized, "invalid_credentials")
	tech3 := e.client()
	if r := tech3.login("teknisi.01", temp2); r.code != http.StatusOK || r.body["user"].(map[string]any)["must_change_password"] != true {
		t.Fatalf("login after reset: %d %v", r.code, r.body)
	}

	// Audit: every step recorded, no temporary password anywhere.
	for _, a := range []string{"user.create", "user.roles.update", "user.update", "user.disable", "user.enable", "user.password.reset"} {
		if countAudit(t, e, a) == 0 {
			t.Errorf("no audit event %s", a)
		}
	}
	var leaked int
	_ = e.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE metadata::text LIKE '%' || $1 || '%'
		OR metadata::text LIKE '%' || $2 || '%'`, temp, temp2).Scan(&leaked)
	if leaked != 0 {
		t.Fatal("temporary password found in audit metadata")
	}
}

// TestUsersValidationAndGuards: input validation, self-protection and the last-Admin guard.
func TestUsersValidationAndGuards(t *testing.T) {
	e := newAuthEnv(t, nil)
	admin := e.signIn("admin.two", rbac.RoleAdmin)
	me := admin.get("/api/v1/auth/me")
	selfID := me.body["user"].(map[string]any)["id"].(string)

	field := func(r resp) string {
		f, _ := r.body["error"].(map[string]any)["field"].(string)
		return f
	}
	for _, tc := range []struct {
		name  string
		body  map[string]any
		field string
	}{
		{"bad username", map[string]any{"username": "a b", "display_name": "x", "roles": []string{"viewer"}}, "username"},
		{"no display name", map[string]any{"username": "abc", "display_name": " ", "roles": []string{"viewer"}}, "display_name"},
		{"bad email", map[string]any{"username": "abc", "display_name": "x", "email": "nope", "roles": []string{"viewer"}}, "email"},
		{"no roles", map[string]any{"username": "abc", "display_name": "x", "roles": []string{}}, "roles"},
		{"unknown role", map[string]any{"username": "abc", "display_name": "x", "roles": []string{"root"}}, "roles"},
		{"duplicate role", map[string]any{"username": "abc", "display_name": "x", "roles": []string{"viewer", "viewer"}}, "roles"},
	} {
		r := admin.post("/api/v1/users", tc.body)
		if r.code != http.StatusUnprocessableEntity || r.errCode() != "validation_failed" || field(r) != tc.field {
			t.Errorf("%s: got %d %q field %q", tc.name, r.code, r.errCode(), field(r))
		}
	}
	want(t, "duplicate username (case-insensitive)", admin.post("/api/v1/users", map[string]any{"username": "ADMIN.TWO",
		"display_name": "x", "roles": []string{"viewer"}}), http.StatusConflict, "username_taken")
	want(t, "unknown id", admin.get("/api/v1/users/00000000-0000-4000-8000-000000000000"), http.StatusNotFound, "not_found")
	want(t, "malformed id", admin.get("/api/v1/users/xyz"), http.StatusNotFound, "not_found")

	want(t, "disable self", admin.post("/api/v1/users/"+selfID+"/disable", nil), http.StatusConflict, "self_action")
	want(t, "drop own admin", admin.put("/api/v1/users/"+selfID+"/roles", map[string]any{"roles": []string{"viewer"}}),
		http.StatusConflict, "self_action")
	want(t, "reset own password", admin.post("/api/v1/users/"+selfID+"/password-reset", nil), http.StatusConflict, "self_action")
	want(t, "add role to self is fine", admin.put("/api/v1/users/"+selfID+"/roles", map[string]any{"roles": []string{"admin", "auditor"}}),
		http.StatusOK, "")

	// Last active Admin: a caller without a principal (system) cannot remove the only Admin's role or disable it.
	ctx := context.Background()
	id := userIDOf(t, me)
	if _, err := e.usr.SetRoles(ctx, mustUUID(t, id), []string{"viewer"}, auditMeta()); !errors.Is(err, usersservice.ErrLastAdmin) {
		t.Fatalf("SetRoles on last admin: %v", err)
	}
	if _, err := e.usr.SetStatus(ctx, mustUUID(t, id), "disabled", auditMeta()); !errors.Is(err, usersservice.ErrLastAdmin) {
		t.Fatalf("disable last admin: %v", err)
	}
	// With a second active Admin it is allowed.
	r := admin.post("/api/v1/users", map[string]any{"username": "admin.three", "display_name": "x", "roles": []string{"admin"}})
	want(t, "second admin", r, http.StatusCreated, "")
	if _, err := e.usr.SetRoles(ctx, mustUUID(t, id), []string{"viewer"}, auditMeta()); err != nil {
		t.Fatalf("SetRoles with another admin: %v", err)
	}
	if countAudit(t, e, "user.roles.update") == 0 {
		t.Error("no user.roles.update audit")
	}
	var denied int
	_ = e.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE result = 'denied' AND metadata->>'reason' IN ('self', 'last_admin')`).Scan(&denied)
	if denied < 4 {
		t.Errorf("denied self/last_admin audit events = %d, want ≥ 4", denied)
	}
}

// TestVersionDetailsByPermission: /version details only for system.version.read (Admin), on the same endpoint.
func TestVersionDetailsByPermission(t *testing.T) {
	e := newAuthEnv(t, nil)
	keys := func(c *client) []string {
		r := c.get("/api/v1/version")
		var k []string
		for key := range r.body {
			k = append(k, key)
		}
		slices.Sort(k)
		return k
	}
	if k := keys(e.client()); !slices.Equal(k, []string{"version"}) {
		t.Errorf("anonymous: %v", k)
	}
	if k := keys(e.signIn("viewer.one", rbac.RoleViewer)); !slices.Equal(k, []string{"version"}) {
		t.Errorf("viewer: %v", k)
	}
	if k := keys(e.signIn("admin.four", rbac.RoleAdmin)); !slices.Contains(k, "commit") || !slices.Contains(k, "server_time_display") {
		t.Errorf("admin: %v", k)
	}
	// A broken session cookie on the public /version is ignored, never an error.
	c := e.client()
	r := c.do(http.MethodGet, "/api/v1/version", nil, func(r *http.Request) {
		r.AddCookie(&http.Cookie{Name: authhandler.SessionCookie, Value: "garbage"})
	})
	if r.code != http.StatusOK {
		t.Errorf("bad cookie: %d", r.code)
	}
}

// TestRolesEndpoint lists the system roles with permissions from code.
func TestRolesEndpoint(t *testing.T) {
	e := newAuthEnv(t, nil)
	admin := e.signIn("admin.five", rbac.RoleAdmin)
	r := admin.get("/api/v1/roles")
	want(t, "roles", r, http.StatusOK, "")
	b, _ := json.Marshal(r.body["roles"])
	var roles []usersservice.Role
	_ = json.Unmarshal(b, &roles)
	if len(roles) != len(rbac.SystemRoleKeys) || roles[0].Key != rbac.RoleAdmin || roles[0].UserCount != 1 {
		t.Fatalf("roles %+v", roles)
	}
	if len(roles[0].Permissions) != len(rbac.All) {
		t.Fatalf("admin permissions %v", roles[0].Permissions)
	}
}

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func auditMeta() auditservice.Meta { return auditservice.Meta{} }
