//go:build integration

package integration

import (
	"context"
	"encoding/csv"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	audithandler "github.com/agungharyanto/sentra-dcim/internal/audit/handler"
)

// getRaw fetches a non-JSON response (CSV export): status, headers, body.
func (c *client) getRaw(t *testing.T, p string) (int, http.Header, string) {
	t.Helper()
	u := *c.url
	ref, _ := url.Parse(p)
	res, err := c.hc.Do(&http.Request{Method: http.MethodGet, URL: u.ResolveReference(ref), Header: http.Header{}})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, res.Header, string(b)
}

func userAgent(ua string) reqOpt { return func(r *http.Request) { r.Header.Set("User-Agent", ua) } }

func TestAuditViewerPermissionsFiltersAndPaging(t *testing.T) {
	e := newAuthEnv(t, nil)
	admin := e.signIn("view.admin", "admin")
	auditor := e.signIn("view.auditor", "auditor")
	tech := e.signIn("view.tech", "technician")

	want(t, "technician cannot read", tech.get("/api/v1/audit/events"), http.StatusForbidden, "forbidden")
	want(t, "technician cannot export", tech.get("/api/v1/audit/events/export.csv"), http.StatusForbidden, "forbidden")
	want(t, "technician cannot verify", tech.post("/api/v1/audit/integrity/verify", nil), http.StatusForbidden, "forbidden")
	want(t, "auditor reads", auditor.get("/api/v1/audit/events"), http.StatusOK, "")
	want(t, "admin reads", admin.get("/api/v1/audit/events"), http.StatusOK, "")

	for i := 0; i < 7; i++ {
		e.client().loginPassword("nobody.here", "wrong-password-123")
	}

	// Keyset pagination: newest first, no gaps or duplicates, stable under new inserts.
	var seqs []int64
	path := "/api/v1/audit/events?limit=4"
	for pages := 0; ; pages++ {
		r := auditor.get(path)
		want(t, "page", r, http.StatusOK, "")
		for _, ev := range r.body["events"].([]any) {
			seqs = append(seqs, int64(ev.(map[string]any)["seq"].(float64)))
		}
		if pages == 0 {
			e.client().loginPassword("nobody.later", "wrong-password-123") // must not shift later pages
		}
		nb, ok := r.body["next_before"].(float64)
		if !ok {
			break
		}
		path = "/api/v1/audit/events?limit=4&before=" + strconv.FormatInt(int64(nb), 10)
	}
	for i := 1; i < len(seqs); i++ {
		if seqs[i] != seqs[i-1]-1 {
			t.Fatalf("pages not contiguous: %v", seqs)
		}
	}
	if seqs[len(seqs)-1] != 1 {
		t.Fatalf("last page does not reach seq 1: %v", seqs)
	}

	// Filters.
	r := auditor.get("/api/v1/audit/events?action=auth.login&result=failure&actor=nobody.here&limit=200")
	evs := r.body["events"].([]any)
	if len(evs) != 7 {
		t.Fatalf("filtered events = %d, want 7", len(evs))
	}
	first := evs[0].(map[string]any)
	if first["action"] != "auth.login.failure" || first["actor_username"] != "nobody.here" || first["metadata"] == nil {
		t.Fatalf("event = %v", first)
	}
	if r := auditor.get("/api/v1/audit/events?actor=nobody%25"); len(r.body["events"].([]any)) != 0 {
		t.Fatal("actor filter treats % as a wildcard")
	}
	future := url.QueryEscape(time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
	if r := auditor.get("/api/v1/audit/events?since=" + future); len(r.body["events"].([]any)) != 0 {
		t.Fatal("since filter ignored")
	}
	for _, q := range []string{"since=yesterday", "result=maybe", "action=DROP%20TABLE", "before=-1", "limit=500",
		"since=2026-10-14T10:00:00Z&until=2026-10-14T09:00:00Z"} {
		want(t, q, auditor.get("/api/v1/audit/events?"+q), http.StatusUnprocessableEntity, "validation_failed")
	}
	acts := strs(auditor.get("/api/v1/audit/actions").body["actions"])
	if !slices.Contains(acts, "auth.login.failure") || !slices.IsSorted(acts) {
		t.Fatalf("actions = %v", acts)
	}
}

func TestAuditCSVExportNeutralisesFormulasAndIsAudited(t *testing.T) {
	e := newAuthEnv(t, nil)
	auditor := e.signIn("csv.auditor", "auditor")
	// Attacker-controlled fields: the User-Agent header and the attempted username of a failed login (only valid
	// usernames are stored, and those may start with "-").
	payloads := []string{"=HYPERLINK(\"http://evil\",\"x\")", "+1+1", "-2+3", "@SUM(A1)"}
	for _, p := range payloads {
		e.client().post("/api/v1/auth/login", map[string]string{"username": "-1.user", "password": "wrong-password-123"}, userAgent(p))
	}

	code, hdr, body := auditor.getRaw(t, "/api/v1/audit/events/export.csv?action=auth.login.failure")
	if code != http.StatusOK || !strings.HasPrefix(hdr.Get("Content-Type"), "text/csv") ||
		!strings.Contains(hdr.Get("Content-Disposition"), "attachment") || hdr.Get("Cache-Control") != "no-store" {
		t.Fatalf("export response: %d %v", code, hdr)
	}
	recs, err := csv.NewReader(strings.NewReader(body)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(recs[0], audithandler.CSVColumns) || len(recs) != 1+len(payloads) {
		t.Fatalf("csv header/rows: %v (%d rows)", recs[0], len(recs))
	}
	col := func(name string) int { return slices.Index(audithandler.CSVColumns, name) }
	var gotUAs []string
	for _, rec := range recs[1:] {
		for i, cell := range rec {
			if cell != "" && strings.ContainsRune("=+-@\t\r", rune(cell[0])) {
				t.Errorf("column %s starts with a formula character: %q", audithandler.CSVColumns[i], cell)
			}
		}
		gotUAs = append(gotUAs, rec[col("user_agent")])
		if rec[col("actor_username")] != "'-1.user" {
			t.Errorf("actor_username cell = %q", rec[col("actor_username")])
		}
		if !strings.HasSuffix(rec[col("occurred_at_local")], "UTC") { // test env display TZ = UTC
			t.Errorf("local time = %q", rec[col("occurred_at_local")])
		}
	}
	for _, p := range payloads {
		if !slices.Contains(gotUAs, "'"+p) {
			t.Errorf("payload %q not exported as text: %q", p, gotUAs)
		}
	}

	// The export is audited (actor, row count, filters) and is not part of its own output.
	r := auditor.get("/api/v1/audit/events?action=audit.export")
	evs := r.body["events"].([]any)
	if len(evs) != 1 {
		t.Fatalf("audit.export events = %d", len(evs))
	}
	ev := evs[0].(map[string]any)
	md := ev["metadata"].(map[string]any)
	if ev["actor_username"] != "csv.auditor" || md["rows"] != float64(len(payloads)) ||
		md["filters"].(map[string]any)["action"] != "auth.login.failure" || ev["result"] != "success" {
		t.Fatalf("audit.export event = %v", ev)
	}
}

func TestAuditIntegrityEndpoint(t *testing.T) {
	e := newAuthEnv(t, nil)
	admin := e.signIn("integ.admin", "admin")
	r := admin.get("/api/v1/audit/integrity")
	want(t, "integrity", r, http.StatusOK, "")
	if r.body["last"] != nil || r.body["running"] != false {
		t.Fatalf("before first run: %v", r.body)
	}
	v := admin.post("/api/v1/audit/integrity/verify", nil)
	want(t, "verify", v, http.StatusOK, "")
	if v.body["ok"] != true || v.body["checked"].(float64) < 1 {
		t.Fatalf("verify = %v", v.body)
	}
	if last := admin.get("/api/v1/audit/integrity").body["last"].(map[string]any); last["ok"] != true {
		t.Fatalf("last = %v", last)
	}

	// Tamper as the owner → the next verification reports the broken row, and records a failed audit.verify.
	ctx := context.Background()
	if _, err := e.pool.Exec(ctx, `ALTER TABLE audit_events DISABLE TRIGGER audit_events_no_update_delete;
		UPDATE audit_events SET actor_username = 'someone-else' WHERE chain_seq = 2;
		ALTER TABLE audit_events ENABLE TRIGGER audit_events_no_update_delete`); err != nil {
		t.Fatal(err)
	}
	v = admin.post("/api/v1/audit/integrity/verify", nil)
	if v.body["ok"] != false || v.body["broken_seq"] != float64(2) {
		t.Fatalf("tampered verify = %v", v.body)
	}
	var n int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE action = 'audit.verify' AND result = 'failure'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("failed audit.verify events = %d %v", n, err)
	}
}
