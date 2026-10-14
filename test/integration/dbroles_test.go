//go:build integration

package integration

import (
	"context"
	"errors"
	"slices"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	"github.com/agungharyanto/sentra-dcim/internal/platform/migrator"
)

// appGrants is the reviewed privilege set of the application role per table (ISS-039). A new table without an entry
// fails TestAppRoleGrants: decide its grants in its migration and record them here.
var appGrants = map[string][]string{
	"allowed_ips":        {"DELETE", "INSERT", "SELECT", "UPDATE"},
	"audit_checkpoints":  {"SELECT"},
	"audit_events":       {"INSERT", "SELECT"},
	"mfa_recovery_codes": {"DELETE", "INSERT", "SELECT", "UPDATE"},
	"ops_runs":           {"SELECT"},
	"roles":              {"SELECT"},
	"schema_migrations":  {"SELECT"},
	"sessions":           {"DELETE", "INSERT", "SELECT", "UPDATE"},
	"settings":           {"DELETE", "INSERT", "SELECT", "UPDATE"},
	"system_info":        {"SELECT"},
	"user_mfa":           {"DELETE", "INSERT", "SELECT", "UPDATE"},
	"user_preferences":   {"DELETE", "INSERT", "SELECT", "UPDATE"},
	"user_roles":         {"DELETE", "INSERT", "SELECT", "UPDATE"},
	"users":              {"INSERT", "SELECT", "UPDATE"},
}

func migrated(t *testing.T) string {
	t.Helper()
	dsn := tempDB(t)
	m, err := migrator.New(dsn, logging.Discard())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()
	if err := m.Up(); err != nil {
		t.Fatal(err)
	}
	return dsn
}

// TestAppRoleGrants: every table has an explicit grant decision, and the effective privileges of sentra_app match it
// (no TRUNCATE, REFERENCES or TRIGGER anywhere).
func TestAppRoleGrants(t *testing.T) {
	dsn := migrated(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	rows, err := conn.Query(ctx, `SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	for _, tbl := range tables {
		want, ok := appGrants[tbl]
		if !ok {
			t.Errorf("table %s has no reviewed grant decision for sentra_app (add it to appGrants and its migration)", tbl)
			continue
		}
		var got []string
		for _, p := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
			var has bool
			if err := conn.QueryRow(ctx, "SELECT has_table_privilege('sentra_app', $1, $2)", "public."+tbl, p).Scan(&has); err != nil {
				t.Fatal(err)
			}
			if has {
				got = append(got, p)
			}
		}
		sort.Strings(got)
		if !slices.Equal(got, want) {
			t.Errorf("sentra_app on %s: %v, want %v", tbl, got, want)
		}
	}
}

// TestAppRoleCannotAlterAudit: as the application role, the audit log accepts INSERT/SELECT and refuses every way to
// change or remove history — by privileges, not only by the append-only trigger.
func TestAppRoleCannotAlterAudit(t *testing.T) {
	dsn := migrated(t)
	ctx := context.Background()
	app, err := pgx.Connect(ctx, appRoleDSN(t, dsn))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = app.Close(ctx) }()
	if _, err := app.Exec(ctx, `INSERT INTO audit_events (actor_type, action, result) VALUES ('system', 'test.app_role', 'success')`); err != nil {
		t.Fatalf("app role cannot append: %v", err)
	}
	var n int
	if err := app.QueryRow(ctx, `SELECT count(*) FROM audit_events`).Scan(&n); err != nil || n == 0 {
		t.Fatalf("app role cannot read: %v", err)
	}
	for _, stmt := range []string{
		`UPDATE audit_events SET result = 'failure'`,
		`DELETE FROM audit_events`,
		`TRUNCATE audit_events`,
		`ALTER TABLE audit_events DISABLE TRIGGER audit_events_no_update_delete`,
		`ALTER TABLE audit_events ADD COLUMN x int`,
		`DROP TABLE audit_events`,
		`DROP TRIGGER audit_events_no_update_delete ON audit_events`,
		`CREATE TABLE evil (x int)`,
		`DELETE FROM users`,
	} {
		_, err := app.Exec(ctx, stmt)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || (pgErr.Code != "42501" && pgErr.Code != "42P01") {
			t.Errorf("%q as app role: %v, want permission denied (42501)", stmt, err)
		}
	}
	// A non-owner GRANT only warns ("no privileges were granted"): nothing may change.
	_, _ = app.Exec(ctx, `GRANT ALL ON audit_events TO PUBLIC`)
	var leaked bool
	if err := app.QueryRow(ctx, `SELECT has_table_privilege('public', 'audit_events', 'UPDATE')`).Scan(&leaked); err != nil || leaked {
		t.Fatalf("PUBLIC gained UPDATE on audit_events (%v)", err)
	}
}
