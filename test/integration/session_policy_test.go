//go:build integration

package integration

import (
	"context"
	"net/http"
	"net/netip"
	"testing"
	"time"

	auditservice "github.com/agungharyanto/sentra-dcim/internal/audit/service"
)

// idleOf reads a user's newest session deadlines from the database.
func (e *authEnv) idleOf(t *testing.T, username string) (idle, abs time.Time) {
	t.Helper()
	if err := e.pool.QueryRow(context.Background(), `SELECT s.idle_expires_at, s.expires_at FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE u.username = $1 AND s.revoked_at IS NULL ORDER BY s.created_at DESC LIMIT 1`, username).Scan(&idle, &abs); err != nil {
		t.Fatal(err)
	}
	return idle, abs
}

// ageSession pretends the last request was 2 minutes ago (so the next one may write) and the idle deadline is near.
func (e *authEnv) ageSession(t *testing.T, username string) {
	t.Helper()
	e.exec(`UPDATE sessions SET last_seen_at = now() - interval '2 minutes', idle_expires_at = now() + interval '3 minutes'
		WHERE user_id = (SELECT id FROM users WHERE username = $1) AND revoked_at IS NULL`, username)
}

func TestSessionIdleOnlyExtendedByActivity(t *testing.T) {
	e := newAuthEnv(t, nil)
	admin := e.signIn("idle.admin", "admin")

	// Policy from Settings: 10 min idle, 2 h absolute for new sessions; idle above absolute refused.
	want(t, "idle > absolute", admin.put("/api/v1/settings/session", map[string]any{"values": map[string]any{"idle_minutes": 180, "absolute_hours": 2}}),
		http.StatusUnprocessableEntity, "validation_failed")
	want(t, "below minimum", admin.put("/api/v1/settings/session", map[string]any{"values": map[string]any{"idle_minutes": 1}}),
		http.StatusUnprocessableEntity, "validation_failed")
	want(t, "policy", admin.put("/api/v1/settings/session", map[string]any{"values": map[string]any{"idle_minutes": 10, "absolute_hours": 2}}), http.StatusOK, "")
	time.Sleep(11 * time.Second) // the policy cache (10 s) expires
	c := e.signIn("idle.user", "viewer")
	idle, abs := e.idleOf(t, "idle.user")
	if d := time.Until(abs); d < 119*time.Minute || d > 121*time.Minute {
		t.Fatalf("absolute = %v, want ~2 h", d)
	}
	if d := time.Until(idle); d < 9*time.Minute || d > 11*time.Minute {
		t.Fatalf("idle = %v, want ~10 min", d)
	}

	// Reads (background polling) never move the idle deadline.
	e.ageSession(t, "idle.user")
	before, _ := e.idleOf(t, "idle.user")
	for i := 0; i < 3; i++ {
		want(t, "poll", c.get("/api/v1/auth/me"), http.StatusOK, "")
	}
	if after, _ := e.idleOf(t, "idle.user"); !after.Equal(before) {
		t.Fatalf("GET extended idle: %v → %v", before, after)
	}
	// The activity signal does, and returns the new deadline.
	r := c.post("/api/v1/auth/activity", nil)
	want(t, "activity", r, http.StatusOK, "")
	got, err := time.Parse(time.RFC3339Nano, r.body["session"].(map[string]any)["idle_expires_at"].(string))
	if err != nil || time.Until(got) < 9*time.Minute {
		t.Fatalf("activity idle = %v %v", got, err)
	}
	if r.body["server_time"] == nil {
		t.Fatal("activity without server_time")
	}
	// A write request counts as activity too.
	e.ageSession(t, "idle.user")
	c.post("/api/v1/auth/password", map[string]string{"current_password": "wrong-password-x", "new_password": pwNew})
	if after, _ := e.idleOf(t, "idle.user"); time.Until(after) < 9*time.Minute {
		t.Fatalf("write request did not extend idle: %v", time.Until(after))
	}
	// The activity endpoint is not audited (heartbeat) and needs a session.
	if n := e.count(t, `action LIKE '%activity%'`); n != 0 {
		t.Fatalf("activity audited: %d", n)
	}
	want(t, "activity without session", e.client().post("/api/v1/auth/activity", nil), http.StatusUnauthorized, "unauthenticated")
}

func TestPasswordExpiryAndHistory(t *testing.T) {
	e := newAuthEnv(t, nil)
	ctx := context.Background()
	admin := e.signIn("pw.admin", "admin")
	want(t, "expiry 0 or ≥30", admin.put("/api/v1/settings/password", map[string]any{"values": map[string]any{"expiry_days": 10}}),
		http.StatusUnprocessableEntity, "validation_failed")
	want(t, "warn < expiry", admin.put("/api/v1/settings/password", map[string]any{"values": map[string]any{"expiry_days": 30, "warn_days": 30}}),
		http.StatusUnprocessableEntity, "validation_failed")
	want(t, "policy", admin.put("/api/v1/settings/password", map[string]any{"values": map[string]any{"expiry_days": 30, "warn_days": 7, "history": 3}}), http.StatusOK, "")
	time.Sleep(11 * time.Second)

	// Operator: 25 days old → warning; 31 days → expired, forced change.
	op := e.signIn("pw.operator", "technician")
	e.exec(`UPDATE users SET password_changed_at = now() - interval '25 days' WHERE username = 'pw.operator'`)
	me := op.get("/api/v1/auth/me").body["user"].(map[string]any)
	if me["password_expires_soon"] != true || me["password_expired"] != false || me["password_expires_at"] == nil {
		t.Fatalf("25 days: %v", me)
	}
	e.exec(`UPDATE users SET password_changed_at = now() - interval '31 days' WHERE username = 'pw.operator'`)
	me = op.get("/api/v1/auth/me").body["user"].(map[string]any)
	if me["password_expired"] != true || me["must_change_password"] != true {
		t.Fatalf("31 days: %v", me)
	}
	want(t, "blocked until changed", op.get("/api/v1/users"), http.StatusForbidden, "password_change_required")

	// Viewer-only accounts never expire.
	v := e.signIn("pw.viewer", "viewer")
	e.exec(`UPDATE users SET password_changed_at = now() - interval '400 days' WHERE username = 'pw.viewer'`)
	if me := v.get("/api/v1/auth/me").body["user"].(map[string]any); me["password_expires_at"] != nil || me["must_change_password"] != false {
		t.Fatalf("viewer: %v", me)
	}

	// History (3 = current + 2 previous): pwInitial → p1 → p2; then pwInitial and p1 are refused, p3 accepted.
	p1, p2, p3 := "first-"+randHex(8), "second-"+randHex(8), "third-"+randHex(8)
	change := func(cur, next string) resp {
		return op.post("/api/v1/auth/password", map[string]string{"current_password": cur, "new_password": next})
	}
	want(t, "→p1", change(pwInitial, p1), http.StatusNoContent, "")
	want(t, "→p2", change(p1, p2), http.StatusNoContent, "")
	want(t, "reuse initial", change(p2, pwInitial), http.StatusUnprocessableEntity, "password_reused")
	want(t, "reuse p1", change(p2, p1), http.StatusUnprocessableEntity, "password_reused")
	want(t, "→p3", change(p2, p3), http.StatusNoContent, "")
	want(t, "after change: not expired", op.get("/api/v1/users"), http.StatusForbidden, "forbidden") // technician: no users.read
	var n int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM password_history h JOIN users u ON u.id = h.user_id WHERE u.username = 'pw.operator'`).Scan(&n); err != nil || n != 3 {
		t.Fatalf("history rows = %d %v (trimmed to the policy)", n, err)
	}
	if c := e.count(t, `action = 'auth.password.change' AND result = 'failure' AND metadata->>'reason' = 'reused'`); c != 2 {
		t.Fatalf("reuse refusals audited = %d", c)
	}
	// Hashes only: no history row contains a password.
	var leaked int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM password_history WHERE password_hash NOT LIKE '$argon2id$%' OR password_hash LIKE '%'||$1||'%'`, p1).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("history not argon2 hashes only: %d %v", leaked, err)
	}
	// An admin reset puts the replaced password into the history too.
	opID := admin.get("/api/v1/users?q=pw.operator").body["users"].([]any)[0].(map[string]any)["id"].(string)
	rs := admin.post("/api/v1/users/"+opID+"/password-reset", nil)
	want(t, "reset", rs, http.StatusOK, "")
	temp := rs.body["temporary_password"].(string)
	op2 := e.client()
	want(t, "login temp", op2.login("pw.operator", temp), http.StatusOK, "")
	want(t, "p3 refused after reset (in history)", op2.post("/api/v1/auth/password", map[string]string{"current_password": temp, "new_password": p3}),
		http.StatusUnprocessableEntity, "password_reused")
}

func TestKioskSessions(t *testing.T) {
	e := newAuthEnv(t, nil)
	ctx := context.Background()
	admin := e.signIn("kiosk.admin", "admin")
	e.createUser("kiosk.wall", true)
	e.setRoles("kiosk.wall", "viewer")
	id := admin.get("/api/v1/users?q=kiosk.wall").body["users"].([]any)[0].(map[string]any)["id"].(string)
	techID := userIDOf(t, admin.post("/api/v1/users", map[string]any{"username": "kiosk.tech", "display_name": "T", "roles": []string{"technician"}}))

	// Precondition: Allowed IP in use.
	want(t, "allowlist off", admin.put("/api/v1/users/"+id+"/kiosk", map[string]any{"enabled": true}), http.StatusConflict, "kiosk_requires_allowlist")
	want(t, "monitor without entries", admin.put("/api/v1/settings/allowed-ips/mode", map[string]any{"mode": "monitor"}), http.StatusOK, "")
	want(t, "still refused (empty list)", admin.put("/api/v1/users/"+id+"/kiosk", map[string]any{"enabled": true}), http.StatusConflict, "kiosk_requires_allowlist")
	want(t, "add 127.0.0.1", admin.post("/api/v1/settings/allowed-ips", map[string]any{"cidr": "127.0.0.1", "label": "wall"}), http.StatusCreated, "")
	want(t, "viewer only", admin.put("/api/v1/users/"+techID+"/kiosk", map[string]any{"enabled": true}), http.StatusConflict, "kiosk_requires_viewer")
	r := admin.put("/api/v1/users/"+id+"/kiosk", map[string]any{"enabled": true})
	want(t, "enable", r, http.StatusOK, "")
	if r.body["kiosk"] != true {
		t.Fatalf("kiosk flag = %v", r.body["kiosk"])
	}
	want(t, "roles change refused while kiosk", admin.put("/api/v1/users/"+id+"/roles", map[string]any{"roles": []string{"viewer", "auditor"}}),
		http.StatusConflict, "kiosk_requires_viewer")

	// Kiosk sign-in (MFA still required): no idle, no absolute limit.
	k := e.client()
	want(t, "kiosk login", k.login("kiosk.wall", pwInitial), http.StatusOK, "")
	me := k.get("/api/v1/auth/me").body
	if me["session"].(map[string]any)["kiosk"] != true || me["user"].(map[string]any)["kiosk"] != true {
		t.Fatalf("kiosk session = %v", me["session"])
	}
	idle, abs := e.idleOf(t, "kiosk.wall")
	if time.Until(idle) < 10*365*24*time.Hour || time.Until(abs) < 10*365*24*time.Hour {
		t.Fatalf("kiosk deadlines idle=%v abs=%v", idle, abs)
	}

	// Outside the Allowed IP list (127.0.0.1 removed through the CLI path) → session revoked and audited.
	if _, err := e.allow.Remove(ctx, nil, "127.0.0.1/32", netip.Addr{}, auditservice.Meta{}); err != nil {
		t.Fatal(err)
	}
	want(t, "add another so the list is not empty", admin.post("/api/v1/settings/allowed-ips", map[string]any{"cidr": "192.0.2.0/24"}), http.StatusCreated, "")
	want(t, "kiosk from outside", k.get("/api/v1/auth/me"), http.StatusUnauthorized, "unauthenticated")
	if n := e.count(t, `action = 'auth.session.kiosk_revoked' AND metadata->>'reason' = 'ip_not_allowed' AND actor_username = 'kiosk.wall'`); n != 1 {
		t.Fatalf("kiosk revoke audited = %d", n)
	}
	want(t, "stays revoked when back inside", k.get("/api/v1/auth/me"), http.StatusUnauthorized, "unauthenticated")

	// Turning kiosk off ends its sessions.
	want(t, "add back", admin.post("/api/v1/settings/allowed-ips", map[string]any{"cidr": "127.0.0.1"}), http.StatusCreated, "")
	k2 := e.client()
	want(t, "kiosk login again", k2.login("kiosk.wall", pwInitial), http.StatusOK, "")
	want(t, "kiosk ok", k2.get("/api/v1/auth/me"), http.StatusOK, "")
	want(t, "disable kiosk", admin.put("/api/v1/users/"+id+"/kiosk", map[string]any{"enabled": false}), http.StatusOK, "")
	want(t, "session ended", k2.get("/api/v1/auth/me"), http.StatusUnauthorized, "unauthenticated")
	if n := e.count(t, `action = 'user.kiosk' AND result = 'success'`); n != 2 {
		t.Fatalf("user.kiosk events = %d", n)
	}
	if n := e.count(t, `action = 'user.kiosk' AND result = 'denied'`); n < 3 {
		t.Fatalf("refused kiosk changes audited = %d", n)
	}
}
