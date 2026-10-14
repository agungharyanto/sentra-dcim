//go:build integration

package integration

import (
	"context"
	"encoding/csv"
	"net/http"
	"strings"
	"testing"
)

func itemValue(t *testing.T, r resp, name string) (any, bool) {
	t.Helper()
	for _, it := range r.body["items"].([]any) {
		m := it.(map[string]any)
		if m["name"] == name {
			return m["value"], m["is_default"].(bool)
		}
	}
	t.Fatalf("no item %s in %v", name, r.body)
	return nil, false
}

func regionalOf(t *testing.T, c *client) map[string]any {
	t.Helper()
	r := c.get("/api/v1/auth/me")
	want(t, "me", r, http.StatusOK, "")
	reg, ok := r.body["regional"].(map[string]any)
	if !ok {
		t.Fatalf("no regional in /me: %v", r.body)
	}
	return reg
}

func TestSettingsRegionalAPIValidationAndAudit(t *testing.T) {
	e := newAuthEnv(t, nil)
	ctx := context.Background()
	admin := e.signIn("set.admin", "admin")
	auditor := e.signIn("set.auditor", "auditor")

	want(t, "auditor cannot read", auditor.get("/api/v1/settings/regional"), http.StatusForbidden, "forbidden")
	want(t, "auditor cannot write", auditor.put("/api/v1/settings/regional", map[string]any{"values": map[string]any{"time_format": "12h"}}),
		http.StatusForbidden, "forbidden")

	r := admin.get("/api/v1/settings/regional")
	want(t, "get", r, http.StatusOK, "")
	for name, def := range map[string]string{"language": "id", "timezone": "Asia/Jakarta", "time_format": "24h", "date_format": "DD/MM/YYYY"} {
		if v, isDef := itemValue(t, r, name); v != def || !isDef {
			t.Errorf("%s = %v (default %t), want %s", name, v, isDef, def)
		}
	}
	want(t, "unknown group", admin.get("/api/v1/settings/nope"), http.StatusNotFound, "not_found")

	// Validation: nothing is written when any value is invalid.
	for _, bad := range []map[string]any{
		{"timezone": "Mars/Base"},
		{"timezone": "Local"},
		{"timezone": "Asia/Jakarta", "time_format": "13h"},
		{"date_format": "MM/DD/YYYY"},
		{"language": "fr"},
		{"colour": "red"},
		{},
	} {
		want(t, "invalid", admin.put("/api/v1/settings/regional", map[string]any{"values": bad}), http.StatusUnprocessableEntity, "validation_failed")
	}
	want(t, "unknown field", admin.put("/api/v1/settings/regional", map[string]any{"vals": map[string]any{}}), http.StatusBadRequest, "invalid_request")
	if r := admin.put("/api/v1/settings/regional", map[string]any{"values": map[string]any{"timezone": "Mars/Base"}}); r.body["error"].(map[string]any)["field"] != "timezone" {
		t.Fatalf("field = %v", r.body)
	}
	if n := countAudit(t, e, "settings.update"); n != 0 {
		t.Fatalf("settings.update after invalid writes = %d", n)
	}

	r = admin.put("/api/v1/settings/regional", map[string]any{"values": map[string]any{"timezone": "Asia/Makassar", "time_format": "12h"}})
	want(t, "put", r, http.StatusOK, "")
	if v, isDef := itemValue(t, r, "timezone"); v != "Asia/Makassar" || isDef {
		t.Fatalf("timezone = %v", v)
	}
	// Same value again: no change, no event.
	want(t, "put same", admin.put("/api/v1/settings/regional", map[string]any{"values": map[string]any{"timezone": "Asia/Makassar"}}), http.StatusOK, "")
	if n := countAudit(t, e, "settings.update"); n != 2 {
		t.Fatalf("settings.update events = %d, want 2", n)
	}
	var old, newV, actor string
	if err := e.pool.QueryRow(ctx, `SELECT metadata->>'old', metadata->>'new', actor_username FROM audit_events
		WHERE action = 'settings.update' AND target_id = 'regional.timezone'`).Scan(&old, &newV, &actor); err != nil {
		t.Fatal(err)
	}
	if old != "Asia/Jakarta" || newV != "Asia/Makassar" || actor != "set.admin" {
		t.Fatalf("audit old=%s new=%s actor=%s", old, newV, actor)
	}

	// Reset to the default with null.
	r = admin.put("/api/v1/settings/regional", map[string]any{"values": map[string]any{"time_format": nil}})
	if v, isDef := itemValue(t, r, "time_format"); v != "24h" || !isDef {
		t.Fatalf("reset time_format = %v %t", v, isDef)
	}
	if err := e.pool.QueryRow(ctx, `SELECT metadata->>'old', metadata->>'new' FROM audit_events
		WHERE action = 'settings.update' AND target_id = 'regional.time_format' ORDER BY chain_seq DESC LIMIT 1`).Scan(&old, &newV); err != nil || old != "12h" || newV != "24h" {
		t.Fatalf("reset audit old=%s new=%s %v", old, newV, err)
	}

	// The system values drive the bootstrap document and /me of users without preferences.
	b := auditor.get("/api/v1/ui/bootstrap")
	if b.body["timezone"] != "Asia/Makassar" || b.body["default_locale"] != "id" {
		t.Fatalf("bootstrap = %v %v", b.body["timezone"], b.body["default_locale"])
	}
	if reg := regionalOf(t, auditor); reg["timezone"] != "Asia/Makassar" || reg["time_format"] != "24h" || reg["date_format"] != "DD/MM/YYYY" || reg["language"] != "id" {
		t.Fatalf("auditor regional = %v", reg)
	}
}

func TestPreferencesOverrideSystem(t *testing.T) {
	e := newAuthEnv(t, nil)
	admin := e.signIn("pref.admin", "admin")
	viewer := e.signIn("pref.viewer", "viewer")

	want(t, "system", admin.put("/api/v1/settings/regional", map[string]any{"values": map[string]any{"date_format": "YYYY-MM-DD"}}), http.StatusOK, "")

	p := viewer.get("/api/v1/me/preferences")
	want(t, "prefs", p, http.StatusOK, "")
	prefs := p.body["preferences"].(map[string]any)
	if len(prefs) != 4 || prefs["regional.timezone"] != nil {
		t.Fatalf("initial preferences = %v", prefs)
	}
	if p.body["effective"].(map[string]any)["date_format"] != "YYYY-MM-DD" {
		t.Fatalf("effective follows system: %v", p.body["effective"])
	}

	// Viewer overrides two values (no settings permission needed for own preferences).
	p = viewer.put("/api/v1/me/preferences", map[string]any{"values": map[string]any{"regional.timezone": "UTC", "regional.date_format": "DD MMM YYYY", "regional.language": "en"}})
	want(t, "set prefs", p, http.StatusOK, "")
	reg := regionalOf(t, viewer)
	if reg["timezone"] != "UTC" || reg["date_format"] != "DD MMM YYYY" || reg["language"] != "en" || reg["time_format"] != "24h" {
		t.Fatalf("viewer effective = %v", reg)
	}
	// Another user is unaffected; a later system change still reaches keys the viewer did not override.
	if r := regionalOf(t, admin); r["timezone"] != "Asia/Jakarta" || r["date_format"] != "YYYY-MM-DD" {
		t.Fatalf("admin effective = %v", r)
	}
	want(t, "system 12h", admin.put("/api/v1/settings/regional", map[string]any{"values": map[string]any{"time_format": "12h", "timezone": "Asia/Jayapura"}}), http.StatusOK, "")
	if r := regionalOf(t, viewer); r["time_format"] != "12h" || r["timezone"] != "UTC" {
		t.Fatalf("viewer after system change = %v", r)
	}

	// Validation and reset.
	for _, bad := range []map[string]any{{"regional.timezone": "Mars/Base"}, {"timezone": "UTC"}, {"audit.secret": true}} {
		want(t, "bad pref", viewer.put("/api/v1/me/preferences", map[string]any{"values": bad}), http.StatusUnprocessableEntity, "validation_failed")
	}
	p = viewer.put("/api/v1/me/preferences", map[string]any{"values": map[string]any{"regional.timezone": nil}})
	if p.body["preferences"].(map[string]any)["regional.timezone"] != nil || p.body["effective"].(map[string]any)["timezone"] != "Asia/Jayapura" {
		t.Fatalf("after reset = %v", p.body)
	}
	// 3 changes + 1 reset = 4 audited preference changes, attributed to the viewer.
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE action = 'user.preferences.update'
		AND actor_username = 'pref.viewer'`).Scan(&n); err != nil || n != 4 {
		t.Fatalf("user.preferences.update = %d %v", n, err)
	}

	// CSV export uses the exporter's effective time zone for the local column.
	auditor := e.signIn("pref.auditor", "auditor")
	want(t, "auditor pref", auditor.put("/api/v1/me/preferences", map[string]any{"values": map[string]any{"regional.timezone": "Asia/Tokyo"}}), http.StatusOK, "")
	_, _, body := auditor.getRaw(t, "/api/v1/audit/events/export.csv?action=user.preferences")
	recs, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	if err != nil || len(recs) < 2 {
		t.Fatalf("csv: %v %d", err, len(recs))
	}
	if !strings.HasSuffix(recs[1][2], "JST") || !strings.HasSuffix(recs[1][1], "Z") {
		t.Fatalf("csv times utc=%q local=%q", recs[1][1], recs[1][2])
	}
}
