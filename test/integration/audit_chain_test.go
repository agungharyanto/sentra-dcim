//go:build integration

package integration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agungharyanto/sentra-dcim/internal/audit/chain"
	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	"github.com/agungharyanto/sentra-dcim/internal/platform/migrator"
)

// chainEnv: a migrated database, an owner pool (tampering) and an application-role pool (normal writes).
type chainEnv struct {
	owner, app *pgxpool.Pool
}

func newChainEnv(t *testing.T) *chainEnv {
	t.Helper()
	dsn := migrated(t)
	return openChainEnv(t, dsn)
}

func openChainEnv(t *testing.T, dsn string) *chainEnv {
	t.Helper()
	ctx := context.Background()
	owner, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	app, err := pgxpool.New(ctx, appRoleDSN(t, dsn))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.Close(); owner.Close() })
	return &chainEnv{owner: owner, app: app}
}

func (e *chainEnv) insert(t *testing.T, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := e.app.Exec(context.Background(), `
			INSERT INTO audit_events (actor_type, actor_username, action, target_type, target_id, result, ip, user_agent,
			                          request_id, metadata)
			VALUES ('user', 'ünïcode-op', 'test.chain', 'user', $1, 'success', '192.0.2.7', 'Mozilla/5.0 "x"', $2,
			        jsonb_build_object('i', $3::int, 'note', 'a:b|c', 'nested', jsonb_build_object('z', 1, 'a', 2)))`,
			fmt.Sprint(i), fmt.Sprintf("req-%d", i), i); err != nil {
			t.Fatalf("insert as app role: %v", err)
		}
	}
}

// tamper runs SQL as the owner with the append-only trigger paused — what an attacker with DB owner access can do.
func (e *chainEnv) tamper(t *testing.T, sql string, args ...any) {
	t.Helper()
	ctx := context.Background()
	tx, err := e.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, s := range []string{`ALTER TABLE audit_events DISABLE TRIGGER audit_events_no_update_delete`, sql,
		`ALTER TABLE audit_events ENABLE TRIGGER audit_events_no_update_delete`} {
		a := args
		if s != sql {
			a = nil
		}
		if _, err := tx.Exec(ctx, s, a...); err != nil {
			t.Fatalf("tamper %q: %v", s, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// rehash recomputes the whole chain the way the trigger would — a "perfect" rewrite only external anchors expose.
func (e *chainEnv) rehash(t *testing.T) {
	e.tamper(t, `DO $$
		DECLARE r audit_events; prev bytea := decode(repeat('00', 32), 'hex');
		BEGIN
			FOR r IN SELECT * FROM audit_events ORDER BY chain_seq LOOP
				UPDATE audit_events SET prev_hash = prev, hash = sha256(prev || audit_canonical(r)) WHERE id = r.id
				RETURNING hash INTO prev;
			END LOOP;
		END $$`)
}

func mustVerify(t *testing.T, db chain.DB, anchors []chain.Anchor) chain.Result {
	t.Helper()
	res, err := chain.Verify(context.Background(), db, anchors, time.Now())
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	return res
}

func TestAuditChainValidAndTamperDetected(t *testing.T) {
	e := newChainEnv(t)
	e.insert(t, 12)
	res := mustVerify(t, e.app, nil) // SELECT is enough: verification runs as the application role
	if !res.OK || res.Checked != 12 || res.FromSeq != 1 || res.HeadSeq != 12 {
		t.Fatalf("intact chain: %+v", res)
	}

	// The Go recomputation must agree with the SQL trigger on every row (same canonical form).
	var sqlOK bool
	if err := e.owner.QueryRow(context.Background(),
		`SELECT bool_and(hash = sha256(prev_hash || audit_canonical(audit_events))) FROM audit_events`).Scan(&sqlOK); err != nil || !sqlOK {
		t.Fatalf("SQL recomputation disagrees: %v", err)
	}

	e.tamper(t, `UPDATE audit_events SET result = 'failure' WHERE chain_seq = 5`)
	res = mustVerify(t, e.app, nil)
	if res.OK || res.BrokenSeq != 5 || !strings.Contains(res.Reason, "modified") {
		t.Fatalf("modified row 5 not detected: %+v", res)
	}
	if strings.Contains(res.Reason, "failure") {
		t.Fatalf("reason leaks row contents: %q", res.Reason)
	}
}

func TestAuditChainMetadataTamperDetected(t *testing.T) {
	e := newChainEnv(t)
	e.insert(t, 3)
	e.tamper(t, `UPDATE audit_events SET metadata = metadata || '{"i": 99}' WHERE chain_seq = 2`)
	if res := mustVerify(t, e.app, nil); res.OK || res.BrokenSeq != 2 {
		t.Fatalf("metadata change not detected: %+v", res)
	}
}

func TestAuditChainDeletedRowDetected(t *testing.T) {
	e := newChainEnv(t)
	e.insert(t, 6)
	e.tamper(t, `DELETE FROM audit_events WHERE chain_seq = 3`)
	if res := mustVerify(t, e.app, nil); res.OK || res.BrokenSeq != 3 || !strings.Contains(res.Reason, "missing") {
		t.Fatalf("deleted row not detected: %+v", res)
	}
}

func TestAuditChainClientCannotChooseHashes(t *testing.T) {
	e := newChainEnv(t)
	e.insert(t, 2)
	if _, err := e.app.Exec(context.Background(), `
		INSERT INTO audit_events (actor_type, action, result, chain_seq, prev_hash, hash)
		VALUES ('system', 'test.forged', 'success', 1000, decode(repeat('ab', 32), 'hex'), decode(repeat('cd', 32), 'hex'))`); err != nil {
		t.Fatal(err)
	}
	var seq int64
	if err := e.app.QueryRow(context.Background(), `SELECT chain_seq FROM audit_events WHERE action = 'test.forged'`).Scan(&seq); err != nil || seq != 3 {
		t.Fatalf("forged chain_seq kept: %d %v", seq, err)
	}
	if res := mustVerify(t, e.app, nil); !res.OK || res.HeadSeq != 3 {
		t.Fatalf("chain after forged insert: %+v", res)
	}
}

func TestAuditChainConcurrentInsertsNeverFork(t *testing.T) {
	e := newChainEnv(t)
	const workers, each = 8, 25
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < each; i++ {
				if _, err := e.app.Exec(context.Background(),
					`INSERT INTO audit_events (actor_type, action, result, metadata) VALUES ('system', 'test.concurrent', 'success', $1)`,
					fmt.Sprintf(`{"w": %d, "i": %d}`, w, i)); err != nil {
					errs <- err
					return
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent insert: %v", err)
	}
	if res := mustVerify(t, e.app, nil); !res.OK || res.Checked != workers*each {
		t.Fatalf("chain after concurrent inserts: %+v", res)
	}
}

func TestAuditChainAnchorDetectsFullRewrite(t *testing.T) {
	e := newChainEnv(t)
	e.insert(t, 5)
	head, err := chain.Head(context.Background(), e.app)
	if err != nil || head.Seq != 5 {
		t.Fatalf("head: %+v %v", head, err)
	}
	anchors, err := chain.ParseAnchors("{}\n" + fmt.Sprintf(`{"seq":%d,"hash":%q,"at":"2026-10-14T00:00:00Z"}`, head.Seq, head.Hash) + "\n")
	if err != nil || len(anchors) != 1 {
		t.Fatalf("parse anchors: %v %v", anchors, err)
	}
	e.insert(t, 2)
	if res := mustVerify(t, e.app, anchors); !res.OK || res.AnchorsChecked != 1 {
		t.Fatalf("anchored intact chain: %+v", res)
	}

	// The owner edits row 2 and recomputes every hash: internally consistent, but no longer matches the backup.
	e.tamper(t, `UPDATE audit_events SET actor_username = 'someone-else' WHERE chain_seq = 2`)
	e.rehash(t)
	if res := mustVerify(t, e.app, nil); !res.OK {
		t.Fatalf("rewritten chain should be self-consistent (that is why anchors exist): %+v", res)
	}
	if res := mustVerify(t, e.app, anchors); res.OK || res.BrokenSeq != 5 || !strings.Contains(res.Reason, "anchor") {
		t.Fatalf("rewrite not detected by the anchor: %+v", res)
	}

	// Truncating the chain below an anchored row is detected too.
	e2 := newChainEnv(t)
	e2.insert(t, 4)
	h2, _ := chain.Head(context.Background(), e2.app)
	e2.tamper(t, `DELETE FROM audit_events WHERE chain_seq >= 3`)
	if res := mustVerify(t, e2.app, []chain.Anchor{h2}); res.OK || !strings.Contains(res.Reason, "truncated") {
		t.Fatalf("truncation not detected: %+v", res)
	}
}

// TestMigration000007BackfillsExistingRows: rows written before v0.3.4 get a valid chain, and down keeps the rows.
func TestMigration000007BackfillsExistingRows(t *testing.T) {
	ctx := context.Background()
	dsn := tempDB(t)
	m, err := migrator.New(dsn, logging.Discard())
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()
	if err := m.Goto(6); err != nil {
		t.Fatal(err)
	}
	owner, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	for i := 0; i < 4; i++ {
		if _, err := owner.Exec(ctx, `INSERT INTO audit_events (actor_type, action, result, metadata) VALUES ('system', 'test.legacy', 'success', $1)`,
			fmt.Sprintf(`{"i": %d}`, i)); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.Up(); err != nil {
		t.Fatalf("up to 7: %v", err)
	}
	e := openChainEnv(t, dsn)
	e.insert(t, 2)
	if res := mustVerify(t, e.app, nil); !res.OK || res.Checked != 6 {
		t.Fatalf("backfilled chain: %+v", res)
	}
	if err := m.Goto(6); err != nil {
		t.Fatalf("down to 6: %v", err)
	}
	var n int
	if err := owner.QueryRow(ctx, `SELECT count(*) FROM audit_events`).Scan(&n); err != nil || n != 6 {
		t.Fatalf("rows after down: %d %v", n, err)
	}
	if tableExists(t, owner, "audit_checkpoints") {
		t.Fatal("audit_checkpoints should be dropped by down")
	}
	if err := m.Up(); err != nil {
		t.Fatalf("re-up: %v", err)
	}
	if res := mustVerify(t, e.app, nil); !res.OK || res.Checked != 6 {
		t.Fatalf("chain after down/up: %+v", res)
	}
}
