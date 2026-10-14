//go:build integration

package integration

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	allowlistservice "github.com/agungharyanto/sentra-dcim/internal/allowlist/service"
	auditservice "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	"github.com/agungharyanto/sentra-dcim/internal/platform/migrator"
)

// check asks the internal decision endpoint like Caddy's forward_auth does, for client IP ip.
func (e *authEnv) check(t *testing.T, ip, uri string, spoof map[string]string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, e.internal.URL+"/api/v1/allowlist/check", nil)
	req.Header.Set("X-Real-IP", ip)
	req.Header.Set("X-Forwarded-Method", http.MethodGet)
	req.Header.Set("X-Forwarded-Uri", uri)
	for k, v := range spoof {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	return res.StatusCode
}

func (e *authEnv) count(t *testing.T, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE `+sql, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAllowlistModesAndDecision(t *testing.T) {
	e := newAuthEnv(t, nil)
	ctx := context.Background()
	admin := e.signIn("allow.admin", "admin")
	viewer := e.signIn("allow.viewer", "viewer")

	want(t, "viewer cannot read", viewer.get("/api/v1/settings/allowed-ips"), http.StatusForbidden, "forbidden")
	r := admin.get("/api/v1/settings/allowed-ips")
	want(t, "state", r, http.StatusOK, "")
	if r.body["mode"] != "off" || r.body["your_ip"] != "127.0.0.1" || r.body["your_ip_in_list"] != false || r.body["confirm_window_seconds"] != float64(120) {
		t.Fatalf("initial state = %v", r.body)
	}
	if pub := strs(r.body["always_public"]); len(pub) != 2 || pub[1] != "/.well-known/acme-challenge/*" {
		t.Fatalf("always_public = %v", pub)
	}

	// Validation, normalisation, dedup.
	for _, bad := range []string{"", "300.1.1.1", "10.0.0.5/24", "0.0.0.0/0", "10.0.0.0/7", "::/0", "fe80::1%eth0", "a.b.c.d"} {
		want(t, "bad "+bad, admin.post("/api/v1/settings/allowed-ips", map[string]any{"cidr": bad}), http.StatusUnprocessableEntity, "validation_failed")
	}
	want(t, "label too long", admin.post("/api/v1/settings/allowed-ips", map[string]any{"cidr": "192.0.2.0/24", "label": strings.Repeat("x", 101)}),
		http.StatusUnprocessableEntity, "validation_failed")
	want(t, "add v4", admin.post("/api/v1/settings/allowed-ips", map[string]any{"cidr": "192.0.2.0/24", "label": "Kantor Diskominfo"}), http.StatusCreated, "")
	want(t, "add v6", admin.post("/api/v1/settings/allowed-ips", map[string]any{"cidr": "2001:db8::/32", "label": "IPv6"}), http.StatusCreated, "")
	r = admin.post("/api/v1/settings/allowed-ips", map[string]any{"cidr": "::ffff:198.51.100.7"})
	want(t, "mapped v4 single", r, http.StatusCreated, "")
	want(t, "duplicate", admin.post("/api/v1/settings/allowed-ips", map[string]any{"cidr": "198.51.100.7/32"}), http.StatusConflict, "duplicate_network")
	cidrs := []string{}
	var labelID string
	for _, en := range r.body["entries"].([]any) {
		m := en.(map[string]any)
		cidrs = append(cidrs, m["cidr"].(string))
		if m["cidr"] == "192.0.2.0/24" {
			labelID = m["id"].(string)
		}
	}
	if strings.Join(cidrs, ",") != "192.0.2.0/24,198.51.100.7/32,2001:db8::/32" {
		t.Fatalf("entries = %v", cidrs)
	}
	want(t, "label", admin.patch("/api/v1/settings/allowed-ips/"+labelID, map[string]any{"label": "Kantor"}), http.StatusOK, "")
	want(t, "generic PUT refuses managed key", admin.put("/api/v1/settings/allowlist", map[string]any{"values": map[string]any{"mode": "enforce"}}),
		http.StatusUnprocessableEntity, "validation_failed")

	// off: everyone passes, nothing audited.
	if c := e.check(t, "203.0.113.9", "/dashboard", nil); c != http.StatusNoContent {
		t.Fatalf("off: %d", c)
	}

	// monitor: outside passes but is audited (aggregated); inside is not audited.
	want(t, "monitor", admin.put("/api/v1/settings/allowed-ips/mode", map[string]any{"mode": "monitor"}), http.StatusOK, "")
	for i := 0; i < 5; i++ {
		if c := e.check(t, "203.0.113.9", "/dashboard?x=1", nil); c != http.StatusNoContent {
			t.Fatalf("monitor outside: %d", c)
		}
	}
	if c := e.check(t, "192.0.2.44", "/dashboard", nil); c != http.StatusNoContent {
		t.Fatalf("monitor inside: %d", c)
	}
	if n := e.count(t, `action = 'security.allowlist_outside' AND host(ip) = '203.0.113.9' AND target_id = 'GET /dashboard'`); n != 1 {
		t.Fatalf("monitor outside events = %d, want 1 (aggregated, query stripped)", n)
	}
	if n := e.count(t, `action LIKE 'security.allowlist%' AND host(ip) = '192.0.2.44'`); n != 0 {
		t.Fatalf("inside audited: %d", n)
	}
	if n := e.count(t, `action = 'allowlist.mode' AND metadata->>'old' = 'off' AND metadata->>'new' = 'monitor' AND actor_username = 'allow.admin'`); n != 1 {
		t.Fatalf("mode change not audited: %d", n)
	}

	// enforce (CLI --force, so the test client 127.0.0.1 is outside the list): outside blocked, inside allowed,
	// health open; spoofed client headers change nothing.
	if _, err := e.allow.SetMode(ctx, "enforce", allowlistservice.ModeOptions{Force: true}, auditservice.Meta{}); err != nil {
		t.Fatal(err)
	}
	if c := e.check(t, "203.0.113.9", "/", nil); c != http.StatusForbidden {
		t.Fatalf("enforce outside: %d", c)
	}
	for _, ip := range []string{"192.0.2.44", "198.51.100.7", "2001:db8::42"} {
		if c := e.check(t, ip, "/", nil); c != http.StatusNoContent {
			t.Fatalf("enforce inside %s: %d", ip, c)
		}
	}
	if n := e.count(t, `action = 'security.allowlist_blocked' AND host(ip) = '203.0.113.9'`); n != 1 {
		t.Fatalf("blocked events = %d", n)
	}
	// Public listener: the API refuses 127.0.0.1, whatever it claims in X-Forwarded-For / X-Real-IP.
	c := e.client()
	spoof := func(r *http.Request) {
		r.Header.Set("X-Forwarded-For", "192.0.2.44")
		r.Header.Set("X-Real-IP", "192.0.2.44")
		r.Header.Set("True-Client-IP", "192.0.2.44")
	}
	want(t, "enforce: API blocked", c.do(http.MethodGet, "/api/v1/version", nil, spoof), http.StatusForbidden, "ip_not_allowed")
	want(t, "enforce: signed-in admin blocked too", admin.get("/api/v1/auth/me"), http.StatusForbidden, "ip_not_allowed")
	want(t, "enforce: health open", c.get("/api/v1/health"), http.StatusOK, "")

	// The emergency door (service path used by the CLI) restores access.
	if _, err := e.allow.SetMode(ctx, "off", allowlistservice.ModeOptions{}, auditservice.Meta{}); err != nil {
		t.Fatal(err)
	}
	want(t, "off again", admin.get("/api/v1/auth/me"), http.StatusOK, "")
	if n := e.count(t, `action = 'allowlist.mode' AND actor_username = 'cli' AND metadata->>'new' = 'off'`); n != 1 {
		t.Fatalf("cli mode change not audited as cli: %d", n)
	}
}

func TestAllowlistLockoutGuardConfirmAndRollback(t *testing.T) {
	e := newAuthEnv(t, nil)
	ctx := context.Background()
	admin := e.signIn("guard.admin", "admin")
	want(t, "add other", admin.post("/api/v1/settings/allowed-ips", map[string]any{"cidr": "192.0.2.0/24"}), http.StatusCreated, "")

	// The admin (127.0.0.1) is not covered: enforce is refused and nothing changes.
	want(t, "guard", admin.put("/api/v1/settings/allowed-ips/mode", map[string]any{"mode": "enforce"}), http.StatusConflict, "lockout_risk")
	if e.allow.Current().Mode != "off" {
		t.Fatal("mode changed despite the guard")
	}
	if n := e.count(t, `action = 'allowlist.mode' AND result = 'denied'`); n != 1 {
		t.Fatalf("refused enforce not audited: %d", n)
	}

	r := admin.post("/api/v1/settings/allowed-ips", map[string]any{"cidr": "127.0.0.1", "label": "admin"})
	want(t, "add self", r, http.StatusCreated, "")
	if r.body["your_ip_in_list"] != true {
		t.Fatalf("your_ip_in_list = %v", r.body["your_ip_in_list"])
	}
	var selfID string
	for _, en := range r.body["entries"].([]any) {
		if m := en.(map[string]any); m["cidr"] == "127.0.0.1/32" {
			selfID = m["id"].(string)
		}
	}
	start := time.Now()
	r = admin.put("/api/v1/settings/allowed-ips/mode", map[string]any{"mode": "enforce"})
	want(t, "enforce", r, http.StatusOK, "")
	pending, err := time.Parse(time.RFC3339, r.body["pending_until"].(string))
	if err != nil || pending.Before(start.Add(110*time.Second)) || pending.After(start.Add(130*time.Second)) {
		t.Fatalf("pending_until = %v %v", r.body["pending_until"], err)
	}
	// Removing the network that lets the admin in is refused while enforcing; others are fine.
	want(t, "remove self refused", admin.del("/api/v1/settings/allowed-ips/"+selfID), http.StatusConflict, "lockout_risk")
	want(t, "still enforced, admin still in", admin.get("/api/v1/settings/allowed-ips"), http.StatusOK, "")

	// "Keep": confirmation from a connection that passes the enforcement.
	r = admin.post("/api/v1/settings/allowed-ips/confirm", nil)
	want(t, "confirm", r, http.StatusOK, "")
	if r.body["pending_until"] != nil || r.body["mode"] != "enforce" {
		t.Fatalf("after confirm = %v", r.body)
	}
	want(t, "confirm twice", admin.post("/api/v1/settings/allowed-ips/confirm", nil), http.StatusConflict, "no_pending_change")

	// Auto-rollback: a new enforce switch (from monitor) that nobody confirms returns to monitor after 2 minutes.
	want(t, "monitor", admin.put("/api/v1/settings/allowed-ips/mode", map[string]any{"mode": "monitor"}), http.StatusOK, "")
	want(t, "enforce again", admin.put("/api/v1/settings/allowed-ips/mode", map[string]any{"mode": "enforce"}), http.StatusOK, "")
	e.allow.SetClock(func() time.Time { return time.Now().Add(allowlistservice.ConfirmWindow - 5*time.Second) })
	if err := e.allow.Tick(ctx); err != nil || e.allow.Current().Mode != "enforce" {
		t.Fatalf("rolled back too early: %v %s", err, e.allow.Current().Mode)
	}
	e.allow.SetClock(func() time.Time { return time.Now().Add(allowlistservice.ConfirmWindow + time.Second) })
	if err := e.allow.Tick(ctx); err != nil {
		t.Fatal(err)
	}
	if e.allow.Current().Mode != "monitor" || !e.allow.Current().Pending.IsZero() {
		t.Fatalf("after deadline: %+v", e.allow.Current())
	}
	if n := e.count(t, `action = 'allowlist.rollback' AND actor_type = 'system' AND metadata->>'new' = 'monitor'`); n != 1 {
		t.Fatalf("rollback not audited: %d", n)
	}
	// A confirmation after the rollback is refused.
	want(t, "late confirm", admin.post("/api/v1/settings/allowed-ips/confirm", nil), http.StatusConflict, "no_pending_change")
}

func TestAllowlistCLIEmergencyDoor(t *testing.T) {
	if os.Getenv("SENTRA_IT_REDIS_ADDR") == "" {
		t.Skip("SENTRA_IT_REDIS_ADDR not set")
	}
	dsn := tempDB(t)
	m, err := migrator.New(dsn, logging.Discard())
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	_ = m.Close()
	bin := filepath.Join(t.TempDir(), "sentra")
	if out, err := exec.Command("go", "build", "-o", bin, "../../cmd/sentra").CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	env := append(os.Environ(), cliEnv(t, appRoleDSN(t, dsn))...) // the CLI runs in the app container: app role
	run := func(args ...string) (string, error) {
		cmd := exec.Command(bin, append([]string{"allowlist"}, args...)...)
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		return string(out), err
	}
	steps := []struct {
		args []string
		ok   bool
		has  string
	}{
		{[]string{"add", "192.0.2.10", "Ruang", "NOC"}, true, "added 192.0.2.10/32"},
		{[]string{"add", "10.0.0.5/24"}, false, "did you mean 10.0.0.0/24"},
		{[]string{"add", "192.0.2.10/32"}, false, "already"},
		{[]string{"mode", "enforce"}, false, "--force"},
		{[]string{"mode", "enforce", "--force"}, true, "mode: enforce"},
		{[]string{"list"}, true, "Ruang NOC"},
		{[]string{"mode", "off"}, true, "mode: off"},
		{[]string{"remove", "192.0.2.10/32"}, true, "removed 192.0.2.10/32"},
		{[]string{"remove", "192.0.2.10/32"}, false, "not found"},
		{[]string{"mode", "sideways"}, false, "one of off, monitor, enforce"},
	}
	for _, s := range steps {
		out, err := run(s.args...)
		if (err == nil) != s.ok || !strings.Contains(out, s.has) {
			t.Errorf("allowlist %v: err=%v out=%q, want ok=%t containing %q", s.args, err, out, s.ok, s.has)
		}
	}
	conn, err := pgx.Connect(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	var n int
	if err := conn.QueryRow(context.Background(), `SELECT count(*) FROM audit_events WHERE actor_username = 'cli'
		AND actor_type = 'system' AND action LIKE 'allowlist.%'`).Scan(&n); err != nil || n != 4 {
		t.Fatalf("cli audit events = %d %v (add, mode enforce, mode off, remove)", n, err)
	}
}
