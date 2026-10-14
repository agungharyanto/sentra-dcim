//go:build integration

package integration

import (
	"context"
	"encoding/base32"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/agungharyanto/sentra-dcim/internal/api"
	authhandler "github.com/agungharyanto/sentra-dcim/internal/auth/handler"
	"github.com/agungharyanto/sentra-dcim/internal/auth/mfa"
)

// TestAuditCoverage: every unsafe route is registered in api.AuditedRoutes, a realistic session over all of them
// writes every registered action, repeated refusals are aggregated, and no credential ever reaches the audit log.
func TestAuditCoverage(t *testing.T) {
	e := newAuthEnv(t, nil)
	ctx := context.Background()

	// 1. Registry is complete.
	var unsafe []string
	err := chi.Walk(e.srv.Config.Handler.(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions {
			unsafe = append(unsafe, method+" "+strings.ReplaceAll(route, "/*/", "/"))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, rt := range unsafe {
		if len(api.AuditedRoutes[rt]) == 0 {
			t.Errorf("unsafe route %s has no audit actions in api.AuditedRoutes", rt)
		}
	}
	for rt := range api.AuditedRoutes {
		if !slices.Contains(unsafe, rt) {
			t.Errorf("api.AuditedRoutes lists %s, which is not a route", rt)
		}
	}

	// 2. A session touching every route. Secrets seen on the way are collected for step 4.
	secrets := []string{pwInitial, pwNew}
	admin := e.signIn("cov.admin", "admin")
	secrets = append(secrets, admin.cookie(authhandler.SessionCookie), admin.cookie(authhandler.CSRFCookie))

	cr := admin.post("/api/v1/users", map[string]any{"username": "cov.tech", "display_name": "Cov", "roles": []string{"technician"}})
	want(t, "create", cr, http.StatusCreated, "")
	id := userIDOf(t, cr)
	temp, _ := cr.body["temporary_password"].(string)
	secrets = append(secrets, temp)
	want(t, "patch", admin.patch("/api/v1/users/"+id, map[string]any{"display_name": "Cov 2"}), http.StatusOK, "")
	want(t, "roles", admin.put("/api/v1/users/"+id+"/roles", map[string]any{"roles": []string{"technician", "viewer"}}), http.StatusOK, "")
	want(t, "disable", admin.post("/api/v1/users/"+id+"/disable", nil), http.StatusOK, "")
	want(t, "enable", admin.post("/api/v1/users/"+id+"/enable", nil), http.StatusOK, "")
	rs := admin.post("/api/v1/users/"+id+"/password-reset", nil)
	want(t, "reset", rs, http.StatusOK, "")
	temp2, _ := rs.body["temporary_password"].(string)
	secrets = append(secrets, temp2)

	tech, tech2 := e.client(), e.client()
	want(t, "wrong password", tech.loginPassword("cov.tech", "definitely-wrong-pw"), http.StatusUnauthorized, "invalid_credentials")
	want(t, "tech login", tech.login("cov.tech", temp2), http.StatusOK, "")
	want(t, "change", tech.post("/api/v1/auth/password", map[string]string{"current_password": temp2, "new_password": pwNew}),
		http.StatusNoContent, "")
	secret := e.totp["cov.tech"]
	secrets = append(secrets, base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret),
		tech.cookie(authhandler.SessionCookie), tech.cookie(authhandler.CSRFCookie))

	// Wrong MFA code on a second session, then the right one.
	want(t, "tech2 password", tech2.loginPassword("cov.tech", pwNew), http.StatusOK, "")
	want(t, "wrong code", tech2.post("/api/v1/auth/mfa/verify", map[string]string{"code": "000000"}), http.StatusUnauthorized, "invalid_mfa_code")
	tech2.completeMFA("cov.tech")

	e.exec(`UPDATE user_mfa SET last_used_step = 0 WHERE user_id = $1`, id)
	code := mfa.Code(secret, mfa.Step(time.Now()))
	rc := tech.post("/api/v1/auth/mfa/recovery-codes", map[string]string{"code": code})
	want(t, "recovery codes", rc, http.StatusOK, "")
	for _, c := range rc.body["recovery_codes"].([]any) {
		secrets = append(secrets, c.(string))
	}
	sessions := tech.get("/api/v1/auth/sessions").body["sessions"].([]any)
	for _, s := range sessions {
		if m := s.(map[string]any); m["current"] != true {
			if r := tech.del("/api/v1/auth/sessions/" + m["id"].(string)); r.code != http.StatusNoContent {
				t.Fatalf("revoke: %d", r.code)
			}
			break
		}
	}
	want(t, "mfa reset", admin.post("/api/v1/users/"+id+"/mfa-reset", nil), http.StatusNoContent, "")
	want(t, "verify chain", admin.post("/api/v1/audit/integrity/verify", nil), http.StatusOK, "")
	want(t, "settings", admin.put("/api/v1/settings/regional", map[string]any{"values": map[string]any{"time_format": "12h"}}), http.StatusOK, "")
	want(t, "preferences", admin.put("/api/v1/me/preferences", map[string]any{"values": map[string]any{"regional.language": "en"}}), http.StatusOK, "")
	// Allowed IP (v0.3.6): add (self + another), relabel, remove, monitor → enforce → keep, then off.
	al := admin.post("/api/v1/settings/allowed-ips", map[string]any{"cidr": "127.0.0.1", "label": "self"})
	want(t, "allow self", al, http.StatusCreated, "")
	al = admin.post("/api/v1/settings/allowed-ips", map[string]any{"cidr": "192.0.2.0/24"})
	want(t, "allow other", al, http.StatusCreated, "")
	var otherID string
	for _, en := range al.body["entries"].([]any) {
		if m := en.(map[string]any); m["cidr"] == "192.0.2.0/24" {
			otherID = m["id"].(string)
		}
	}
	want(t, "relabel", admin.patch("/api/v1/settings/allowed-ips/"+otherID, map[string]any{"label": "x"}), http.StatusOK, "")
	want(t, "remove", admin.del("/api/v1/settings/allowed-ips/"+otherID), http.StatusOK, "")
	want(t, "enforce", admin.put("/api/v1/settings/allowed-ips/mode", map[string]any{"mode": "enforce"}), http.StatusOK, "")
	want(t, "keep", admin.post("/api/v1/settings/allowed-ips/confirm", nil), http.StatusOK, "")
	want(t, "off", admin.put("/api/v1/settings/allowed-ips/mode", map[string]any{"mode": "off"}), http.StatusOK, "")
	if r := admin.post("/api/v1/auth/logout", nil); r.code != http.StatusNoContent {
		t.Fatalf("logout: %d", r.code)
	}

	// 3. Repeated refusals are aggregated: 25 × 403 and 25 × CSRF failure → one event each (plus summaries later).
	viewer := e.signIn("cov.viewer", "viewer")
	for i := 0; i < 25; i++ {
		viewer.post("/api/v1/users", map[string]any{"username": "x.y.z", "display_name": "x", "roles": []string{"viewer"}})
		viewer.post("/api/v1/auth/password", map[string]string{"current_password": "a", "new_password": "b"}, noCSRF)
	}
	countWhere := func(sql string, args ...any) int {
		var n int
		if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE `+sql, args...).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := countWhere(`action = 'authz.denied' AND actor_username = 'cov.viewer' AND target_id = 'POST /api/v1/users'`); n != 1 {
		t.Errorf("authz.denied rows for 25 repeats = %d, want 1", n)
	}
	if n := countWhere(`action = 'security.csrf_denied' AND actor_username = 'cov.viewer'`); n != 1 {
		t.Errorf("security.csrf_denied rows for 25 repeats = %d, want 1", n)
	}

	// Every registered action was written.
	for rt, actions := range api.AuditedRoutes {
		for _, a := range actions {
			if countWhere(`action = $1`, a) == 0 {
				t.Errorf("%s: no %s event recorded", rt, a)
			}
		}
	}

	// 4. No credential anywhere in the audit log (any column).
	var dump string
	if err := e.pool.QueryRow(ctx, `SELECT coalesce(string_agg(concat_ws('|', actor_username, action, target_type, target_id,
		user_agent, request_id, metadata::text), E'\n'), '') FROM audit_events`).Scan(&dump); err != nil {
		t.Fatal(err)
	}
	for _, s := range secrets {
		if s == "" {
			t.Fatal("a collected secret is empty")
		}
		if strings.Contains(dump, s) {
			t.Errorf("audit log contains a credential (len %d)", len(s))
		}
	}
	for _, c := range []string{code, "000000"} {
		if strings.Contains(dump, `"`+c+`"`) {
			t.Errorf("audit log contains an MFA code")
		}
	}
}
