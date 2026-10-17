//go:build integration

package integration

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	auditservice "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	facilityservice "github.com/agungharyanto/sentra-dcim/internal/facility/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	"github.com/agungharyanto/sentra-dcim/internal/platform/migrator"
	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

func ifMatchHdr(v string) reqOpt { return func(r *http.Request) { r.Header.Set("If-Match", v) } }

func idOf(t *testing.T, r resp) string {
	t.Helper()
	id, _ := r.body["id"].(string)
	if id == "" {
		t.Fatalf("no id in %d %v", r.code, r.body)
	}
	return id
}

func etagOf(t *testing.T, r resp) string {
	t.Helper()
	tag := r.header.Get("ETag")
	if tag == "" {
		t.Fatalf("no ETag (%d %v)", r.code, r.body)
	}
	return tag
}

// facilityTree creates site → building → floor → room through the API and returns their IDs.
type facIDs struct{ site, building, floor, room string }

func makeFacility(t *testing.T, c *client, code string) facIDs {
	t.Helper()
	var ids facIDs
	r := c.post("/api/v1/sites", map[string]any{"code": code, "name": "Site " + code, "address": "Jl. Test"})
	want(t, "site "+code, r, http.StatusCreated, "")
	ids.site = idOf(t, r)
	r = c.post("/api/v1/buildings", map[string]any{"site_id": ids.site, "name": "Gedung A", "code": "GA"})
	want(t, "building", r, http.StatusCreated, "")
	ids.building = idOf(t, r)
	r = c.post("/api/v1/floors", map[string]any{"building_id": ids.building, "name": "Lantai 1", "level": 1})
	want(t, "floor", r, http.StatusCreated, "")
	ids.floor = idOf(t, r)
	r = c.post("/api/v1/rooms", map[string]any{"floor_id": ids.floor, "name": "Ruang Server", "room_type": "whitespace"})
	want(t, "room", r, http.StatusCreated, "")
	ids.room = idOf(t, r)
	return ids
}

// TestFacilityCompositeFK: site_id is denormalised below Site, and the composite foreign keys refuse a row whose
// site_id differs from its parent's — even when written directly by the owner role.
func TestFacilityCompositeFK(t *testing.T) {
	e := newAuthEnv(t, nil)
	admin := e.signIn("fk.admin", "admin")
	a := makeFacility(t, admin, "FKA")
	b := makeFacility(t, admin, "FKB")
	ctx := context.Background()
	cases := map[string]string{
		"floor in building of A with site B": `INSERT INTO floors (building_id, site_id, name) VALUES ('` + a.building + `', '` + b.site + `', 'x')`,
		"room on floor of A with site B":     `INSERT INTO rooms (floor_id, site_id, name) VALUES ('` + a.floor + `', '` + b.site + `', 'x')`,
		"rack in room of A with site B":      `INSERT INTO racks (room_id, site_id, name) VALUES ('` + a.room + `', '` + b.site + `', 'x')`,
		"rack moved to a room of B":          `UPDATE racks SET room_id = '` + b.room + `' WHERE site_id = '` + a.site + `'`,
	}
	r := admin.post("/api/v1/racks", map[string]any{"room_id": a.room, "name": "R1"})
	want(t, "rack", r, http.StatusCreated, "")
	for name, sql := range cases {
		if _, err := e.pool.Exec(ctx, sql); err == nil || !strings.Contains(err.Error(), "foreign key") {
			t.Errorf("%s: want foreign key violation, got %v", name, err)
		}
	}
	// The application role cannot delete facility rows (archive only, D-085).
	var can bool
	if err := e.pool.QueryRow(ctx, "SELECT has_table_privilege('sentra_app', 'racks', 'DELETE')").Scan(&can); err != nil || can {
		t.Errorf("sentra_app DELETE on racks = %t, %v", can, err)
	}
	// Moving a rack to another site through the API: 422 cross_site_move.
	rack := idOf(t, r)
	want(t, "cross-site move", admin.patch2("/api/v1/racks/"+rack, map[string]any{"room_id": b.room}, etagOf(t, r)),
		http.StatusUnprocessableEntity, "cross_site_move")
}

func (c *client) patch2(p string, body any, etag string) resp {
	return c.do(http.MethodPatch, p, body, ifMatchHdr(etag))
}

func (c *client) archive(p, etag string) resp {
	return c.do(http.MethodPost, p+"/archive", nil, ifMatchHdr(etag))
}

// TestFacilityNamesArchiveAndVersions: names unique per parent case-insensitively among active rows and reusable
// after archiving; archiving a parent with active children is 409; PATCH/archive need If-Match (428) with the
// current version (412); a rename is audited as rack.rename with before/after.
func TestFacilityNamesArchiveAndVersions(t *testing.T) {
	e := newAuthEnv(t, nil)
	admin := e.signIn("names.admin", "admin")
	ids := makeFacility(t, admin, "DPS1")

	// Site code: normalised to upper case, unique for ever; site names unique case-insensitively.
	want(t, "dup code", admin.post("/api/v1/sites", map[string]any{"code": "dps1", "name": "Other"}), http.StatusConflict, "duplicate_code")
	want(t, "dup site name", admin.post("/api/v1/sites", map[string]any{"code": "DPS2", "name": "SITE dps1"}), http.StatusConflict, "duplicate_name")
	want(t, "bad code", admin.post("/api/v1/sites", map[string]any{"code": "-x", "name": "X"}), http.StatusUnprocessableEntity, "validation_failed")

	// Rack names: trimmed, validated, unique per site case-insensitively.
	r := admin.post("/api/v1/racks", map[string]any{"room_id": ids.room, "name": "  R-A01  ", "asset_tag": "TAG-1"})
	want(t, "rack", r, http.StatusCreated, "")
	rack, tag := idOf(t, r), etagOf(t, r)
	if r.body["name"] != "R-A01" || r.body["u_height"] != float64(42) || r.body["status"] != "planned" {
		t.Errorf("defaults/trim: %v", r.body)
	}
	for _, bad := range []string{"", " ", "-R1", "R@1", strings.Repeat("x", 65), "Rák"} {
		want(t, "bad rack name "+strconv.Quote(bad), admin.post("/api/v1/racks", map[string]any{"room_id": ids.room, "name": bad}),
			http.StatusUnprocessableEntity, "validation_failed")
	}
	want(t, "dup rack name", admin.post("/api/v1/racks", map[string]any{"room_id": ids.room, "name": "r-a01"}), http.StatusConflict, "duplicate_name")
	want(t, "dup asset tag", admin.post("/api/v1/racks", map[string]any{"room_id": ids.room, "name": "R-A02", "asset_tag": "tag-1"}),
		http.StatusConflict, "duplicate_asset_tag")
	want(t, "u height", admin.post("/api/v1/racks", map[string]any{"room_id": ids.room, "name": "R-X", "u_height": 61}),
		http.StatusUnprocessableEntity, "validation_failed")

	// ETag / If-Match.
	g := admin.get("/api/v1/racks/" + rack)
	want(t, "get rack", g, http.StatusOK, "")
	if g.header.Get("ETag") != tag || tag != `"1"` {
		t.Errorf("ETag = %q / %q, want \"1\"", g.header.Get("ETag"), tag)
	}
	want(t, "no If-Match", admin.patch("/api/v1/racks/"+rack, map[string]any{"name": "R-B01"}), http.StatusPreconditionRequired, "precondition_required")
	want(t, "wildcard If-Match", admin.do(http.MethodPatch, "/api/v1/racks/"+rack, map[string]any{"name": "R-B01"}, ifMatchHdr("*")),
		http.StatusPreconditionRequired, "precondition_required")
	ren := admin.patch2("/api/v1/racks/"+rack, map[string]any{"name": "R-B01", "max_power_kw": 7.5}, tag)
	want(t, "rename", ren, http.StatusOK, "")
	if ren.header.Get("ETag") != `"2"` || ren.body["max_power_kw"] != 7.5 {
		t.Errorf("after rename: ETag %q body %v", ren.header.Get("ETag"), ren.body)
	}
	want(t, "stale If-Match", admin.patch2("/api/v1/racks/"+rack, map[string]any{"name": "R-C01"}, tag), http.StatusPreconditionFailed, "version_conflict")
	var md string
	if err := e.pool.QueryRow(context.Background(), `SELECT metadata::text FROM audit_events WHERE action = 'rack.rename' AND target_id = $1`, rack).Scan(&md); err != nil {
		t.Fatalf("rack.rename audit: %v", err)
	}
	for _, s := range []string{`"before"`, `"after"`, `"R-A01"`, `"R-B01"`, `"max_power_kw"`} {
		if !strings.Contains(md, s) {
			t.Errorf("rename audit metadata %s lacks %s", md, s)
		}
	}
	// A field set to null clears it (asset tag) and is audited as rack.update.
	upd := admin.patch2("/api/v1/racks/"+rack, map[string]any{"asset_tag": nil}, ren.header.Get("ETag"))
	want(t, "clear asset tag", upd, http.StatusOK, "")
	if upd.body["asset_tag"] != nil {
		t.Errorf("asset_tag not cleared: %v", upd.body["asset_tag"])
	}

	// Archive: parents with active children refuse; the name is free again afterwards.
	room := admin.get("/api/v1/rooms/" + ids.room)
	want(t, "room with racks", admin.archive("/api/v1/rooms/"+ids.room, etagOf(t, room)), http.StatusConflict, "has_active_children")
	site := admin.get("/api/v1/sites/" + ids.site)
	want(t, "site with buildings", admin.archive("/api/v1/sites/"+ids.site, etagOf(t, site)), http.StatusConflict, "has_active_children")
	arch := admin.archive("/api/v1/racks/"+rack, upd.header.Get("ETag"))
	want(t, "archive rack", arch, http.StatusOK, "")
	if arch.body["archived_at"] == nil {
		t.Error("archived_at not set")
	}
	want(t, "archive again", admin.archive("/api/v1/racks/"+rack, arch.header.Get("ETag")), http.StatusConflict, "archived")
	want(t, "edit archived", admin.patch2("/api/v1/racks/"+rack, map[string]any{"notes": "x"}, arch.header.Get("ETag")), http.StatusConflict, "archived")
	want(t, "name reusable", admin.post("/api/v1/racks", map[string]any{"room_id": ids.room, "name": "r-b01"}), http.StatusCreated, "")
	tagged := admin.post("/api/v1/racks", map[string]any{"room_id": ids.room, "name": "R-T", "asset_tag": "TAG-2"})
	want(t, "tagged rack", tagged, http.StatusCreated, "")
	want(t, "archive tagged", admin.archive("/api/v1/racks/"+idOf(t, tagged), etagOf(t, tagged)), http.StatusOK, "")
	want(t, "asset tag stays taken after archive", admin.post("/api/v1/racks", map[string]any{"room_id": ids.room, "name": "R-Z", "asset_tag": "tag-2"}),
		http.StatusConflict, "duplicate_asset_tag")

	lst := admin.get("/api/v1/racks?room_id=" + ids.room)
	if lst.body["total"] != float64(1) {
		t.Errorf("active racks = %v", lst.body["total"])
	}
	lst = admin.get("/api/v1/racks?room_id=" + ids.room + "&include_archived=true&sort=-name")
	if lst.body["total"] != float64(3) {
		t.Errorf("racks incl. archived = %v", lst.body["total"])
	}
	want(t, "bad sort", admin.get("/api/v1/racks?sort=drop"), http.StatusUnprocessableEntity, "validation_failed")
	want(t, "bad filter", admin.get("/api/v1/racks?site_id=nope"), http.StatusUnprocessableEntity, "validation_failed")

	// Archive bottom-up: room (after its racks), floor, building, site.
	for _, k := range admin.get("/api/v1/racks?room_id=" + ids.room).body["items"].([]any) {
		m := k.(map[string]any)
		want(t, "archive rack", admin.archive("/api/v1/racks/"+m["id"].(string), `"`+strconv.Itoa(int(m["version"].(float64)))+`"`), http.StatusOK, "")
	}
	for _, p := range []string{"/api/v1/rooms/" + ids.room, "/api/v1/floors/" + ids.floor, "/api/v1/buildings/" + ids.building, "/api/v1/sites/" + ids.site} {
		g := admin.get(p)
		want(t, "archive "+p, admin.archive(p, etagOf(t, g)), http.StatusOK, "")
	}
	want(t, "child of archived", admin.post("/api/v1/buildings", map[string]any{"site_id": ids.site, "name": "B"}), http.StatusUnprocessableEntity, "parent_archived")
	if tr := admin.get("/api/v1/facility/tree"); len(tr.body["sites"].([]any)) != 0 {
		t.Errorf("tree shows archived sites: %v", tr.body)
	}
}

// TestFacilitySiteScope (D-084): a role assigned for one site reaches that site only — lists are filtered, other
// sites answer 404 by ID; the site assignment grants no global permission; only global managers create sites.
func TestFacilitySiteScope(t *testing.T) {
	e := newAuthEnv(t, nil)
	admin := e.signIn("scope.admin", "admin")
	a := makeFacility(t, admin, "SITEA")
	b := makeFacility(t, admin, "SITEB")
	ra := admin.post("/api/v1/racks", map[string]any{"room_id": a.room, "name": "A-01"})
	rb := admin.post("/api/v1/racks", map[string]any{"room_id": b.room, "name": "B-01"})
	want(t, "racks", rb, http.StatusCreated, "")

	// Technician for site A only (no global role).
	tech := e.signIn("scope.tech", "viewer")
	techID := e.userID("scope.tech")
	r := admin.put("/api/v1/users/"+techID+"/site-roles", map[string]any{"site_roles": []map[string]any{{"role": "technician", "site_id": a.site}}})
	want(t, "site role", r, http.StatusOK, "")
	want(t, "clear global roles", admin.put("/api/v1/users/"+techID+"/roles", map[string]any{"roles": []string{}}), http.StatusOK, "")
	if sr := admin.get("/api/v1/users/" + techID).body["site_roles"].([]any); len(sr) != 1 || sr[0].(map[string]any)["site_code"] != "SITEA" {
		t.Errorf("site_roles = %v", sr)
	}
	want(t, "admin per site", admin.put("/api/v1/users/"+techID+"/site-roles", map[string]any{"site_roles": []map[string]any{{"role": "admin", "site_id": a.site}}}),
		http.StatusUnprocessableEntity, "validation_failed")
	want(t, "last role", admin.put("/api/v1/users/"+techID+"/site-roles", map[string]any{"site_roles": []any{}}), http.StatusConflict, "no_roles")
	if _, err := e.pool.Exec(context.Background(), `INSERT INTO user_roles (user_id, role_id, site_id) SELECT $1, id, $2 FROM roles WHERE key = 'admin'`,
		techID, a.site); err == nil {
		t.Error("database accepted a site-scoped admin assignment")
	}

	me := tech.get("/api/v1/auth/me").body["user"].(map[string]any)
	if p := strs(me["permissions"]); !contains(p, "facility.read") || !contains(p, "rack.read") || contains(p, "users.read") {
		t.Errorf("effective permissions = %v", p)
	}
	if g := strs(me["global_permissions"]); len(g) != 0 {
		t.Errorf("global permissions = %v", g)
	}
	sites := tech.get("/api/v1/sites")
	want(t, "list sites", sites, http.StatusOK, "")
	if sites.body["total"] != float64(1) || sites.body["items"].([]any)[0].(map[string]any)["code"] != "SITEA" {
		t.Errorf("scoped site list = %v", sites.body)
	}
	if n := tech.get("/api/v1/racks").body["total"]; n != float64(1) {
		t.Errorf("scoped rack list = %v", n)
	}
	if n := tech.get("/api/v1/racks?site_id=" + b.site).body["total"]; n != float64(0) {
		t.Errorf("rack list filtered to B = %v", n)
	}
	for _, p := range []string{"/api/v1/sites/" + b.site, "/api/v1/buildings/" + b.building, "/api/v1/floors/" + b.floor,
		"/api/v1/rooms/" + b.room, "/api/v1/racks/" + idOf(t, rb)} {
		want(t, "out of scope "+p, tech.get(p), http.StatusNotFound, "not_found")
	}
	want(t, "in scope", tech.get("/api/v1/racks/"+idOf(t, ra)), http.StatusOK, "")
	if tr := tech.get("/api/v1/facility/tree").body["sites"].([]any); len(tr) != 1 || tr[0].(map[string]any)["rack_count"] != float64(1) {
		t.Errorf("scoped tree = %v", tr)
	}
	// Read-only role: no manage permission at all.
	want(t, "tech creates site", tech.post("/api/v1/sites", map[string]any{"code": "X", "name": "X"}), http.StatusForbidden, "forbidden")
	want(t, "tech users", tech.get("/api/v1/users"), http.StatusForbidden, "forbidden")

	// A site-scoped manager (no system role grants it in v0.4.1; future roles may): service level.
	ctx := principal.With(context.Background(), principal.Principal{Type: principal.TypeUser, ID: uuid.New(), Username: "mgr",
		Sites: map[uuid.UUID][]rbac.Permission{uuid.MustParse(a.site): {rbac.FacilityRead, rbac.FacilityManage, rbac.RackRead, rbac.RackManage}}})
	if _, err := e.fac.CreateSite(ctx, facilityservice.SiteInput{Code: "NEW", Name: "New"}, auditservice.Meta{}); !errors.Is(err, facilityservice.ErrGlobalScope) {
		t.Errorf("scoped manager creates site: %v", err)
	}
	if _, err := e.fac.CreateBuilding(ctx, facilityservice.BuildingInput{SiteID: uuid.MustParse(b.site), Name: "X"}, auditservice.Meta{}); err == nil {
		t.Error("scoped manager created a building in another site")
	}
	if _, err := e.fac.CreateBuilding(ctx, facilityservice.BuildingInput{SiteID: uuid.MustParse(a.site), Name: "Gedung B"}, auditservice.Meta{}); err != nil {
		t.Errorf("scoped manager in own site: %v", err)
	}
	st, err := e.fac.GetSite(ctx, uuid.MustParse(a.site))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.fac.ArchiveSite(ctx, st.ID, st.Version, auditservice.Meta{}); !errors.Is(err, facilityservice.ErrGlobalScope) {
		t.Errorf("scoped manager archives site: %v", err)
	}
	if _, err := e.fac.GetSite(ctx, uuid.MustParse(b.site)); !errors.Is(err, facilityservice.ErrNotFound) {
		t.Errorf("scoped manager reads other site: %v", err)
	}
	// Read in B, manage in A: changes in B are 403 out_of_scope, not 404.
	ctx2 := principal.With(context.Background(), principal.Principal{Type: principal.TypeUser, ID: uuid.New(), Username: "mgr2",
		Permissions: []rbac.Permission{rbac.FacilityRead, rbac.RackRead},
		Sites:       map[uuid.UUID][]rbac.Permission{uuid.MustParse(a.site): {rbac.FacilityManage, rbac.RackManage}}})
	sb, _ := e.fac.GetSite(ctx2, uuid.MustParse(b.site))
	if _, err := e.fac.UpdateSite(ctx2, sb.ID, sb.Version, facilityservice.SitePatch{}, auditservice.Meta{}); !errors.Is(err, facilityservice.ErrForbidden) {
		t.Errorf("update readable but unmanaged site: %v", err)
	}
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

func (e *authEnv) userID(username string) string {
	e.t.Helper()
	var id string
	if err := e.pool.QueryRow(context.Background(), "SELECT id FROM users WHERE username = $1", username).Scan(&id); err != nil {
		e.t.Fatal(err)
	}
	return id
}

// TestMigration000011Down: rolling back removes site-scoped assignments (a v0.3 binary would read them as global)
// and restores the user_roles primary key.
func TestMigration000011Down(t *testing.T) {
	e := newAuthEnv(t, nil)
	admin := e.signIn("down.admin", "admin")
	ids := makeFacility(t, admin, "DOWN")
	e.signIn("down.tech", "viewer")
	want(t, "site role", admin.put("/api/v1/users/"+e.userID("down.tech")+"/site-roles",
		map[string]any{"site_roles": []map[string]any{{"role": "technician", "site_id": ids.site}}}), http.StatusOK, "")
	m, err := migrator.New(e.dsn, logging.Discard())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()
	if err := m.Down(1); err != nil {
		t.Fatalf("down 1: %v", err)
	}
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM user_roles ur JOIN users u ON u.id = ur.user_id WHERE u.username = 'down.tech'`).Scan(&n); err != nil || n != 1 {
		t.Errorf("down.tech roles after down = %d (%v), want only the global viewer", n, err)
	}
	var pk string
	if err := e.pool.QueryRow(context.Background(), `SELECT conname FROM pg_constraint WHERE conrelid = 'user_roles'::regclass AND contype = 'p'`).Scan(&pk); err != nil {
		t.Errorf("primary key not restored: %v", err)
	}
	if tableExists(t, e.pool, "racks") || tableExists(t, e.pool, "sites") {
		t.Error("facility tables left after down")
	}
	if err := m.Up(); err != nil {
		t.Fatalf("re-up: %v", err)
	}
}
