//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/agungharyanto/sentra-dcim/internal/audit/chain"
)

// insertAged adds n events dated `age` ago (as the app role).
func (e *chainEnv) insertAged(t *testing.T, n int, age time.Duration) {
	t.Helper()
	for i := 0; i < n; i++ {
		if _, err := e.app.Exec(context.Background(),
			`INSERT INTO audit_events (occurred_at, actor_type, action, result, metadata) VALUES ($1, 'system', 'test.aged', 'success', $2)`,
			time.Now().Add(-age), map[string]any{"i": i, "formula": "=cmd"}); err != nil {
			t.Fatal(err)
		}
	}
}

func (e *chainEnv) prune(t *testing.T, req chain.PruneRequest) (chain.Segment, error) {
	t.Helper()
	ctx := context.Background()
	tx, err := e.owner.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	seg, err := chain.Prune(ctx, tx, req, time.Now())
	if err != nil {
		return seg, err
	}
	return seg, tx.Commit(ctx)
}

func (e *chainEnv) count(t *testing.T) int {
	t.Helper()
	var n int
	if err := e.owner.QueryRow(context.Background(), `SELECT count(*) FROM audit_events`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestAuditPruneWithCheckpointKeepsChainVerifiable(t *testing.T) {
	ctx := context.Background()
	e := newChainEnv(t)
	const old = 400 * 24 * time.Hour
	e.insertAged(t, 5, old)
	e.insert(t, 3)
	preHead, _ := chain.Head(ctx, e.app) // seq 8: an anchor from a backup taken before pruning
	var anchor5 []byte
	if err := e.app.QueryRow(ctx, `SELECT hash FROM audit_events WHERE chain_seq = 5`).Scan(&anchor5); err != nil {
		t.Fatal(err)
	}

	if _, err := chain.PlanPrune(ctx, e.app, 30, time.Now()); err == nil {
		t.Fatal("retention below the minimum accepted")
	}
	plan, err := chain.PlanPrune(ctx, e.app, 365, time.Now())
	if err != nil || plan.FromSeq != 1 || plan.ThroughSeq != 5 || plan.Rows != 5 {
		t.Fatalf("plan: %+v %v", plan, err)
	}

	var archive bytes.Buffer
	seg, err := chain.Export(ctx, e.app, plan.ThroughSeq, &archive) // export needs SELECT only
	if err != nil || seg.Rows != 5 {
		t.Fatalf("export: %+v %v", seg, err)
	}
	again, _ := chain.Export(ctx, e.app, plan.ThroughSeq, &bytes.Buffer{})
	if again.SHA256 != seg.SHA256 {
		t.Fatal("export is not deterministic")
	}
	check, err := chain.VerifyArchive(bytes.NewReader(archive.Bytes()))
	if err != nil || check.Rows != 5 || check.FirstPrev != strings.Repeat("00", 32) || check.LastHash != hex.EncodeToString(anchor5) {
		t.Fatalf("archive check: %+v %v", check, err)
	}

	req := chain.PruneRequest{ThroughSeq: 5, ArchiveName: "audit-000000000001-000000000005.jsonl.gz", ArchiveSHA256: seg.SHA256, RetentionDays: 365}

	// Guards: nothing is deleted when any of them trips.
	bad := req
	bad.ArchiveSHA256 = strings.Repeat("0", 64)
	if _, err := e.prune(t, bad); !errors.Is(err, chain.ErrArchiveMismatch) {
		t.Fatalf("wrong archive checksum: %v", err)
	}
	bad = req
	bad.ThroughSeq = 6
	if _, err := e.prune(t, bad); err == nil || !strings.Contains(err.Error(), "retention window") {
		t.Fatalf("pruning inside retention: %v", err)
	}
	bad = req
	bad.ArchiveName = "../../etc/passwd"
	if _, err := e.prune(t, bad); err == nil {
		t.Fatal("path-like archive name accepted")
	}
	if n := e.count(t); n != 8 {
		t.Fatalf("rows after refused prunes: %d", n)
	}

	// The application role can never prune.
	appTx, err := e.app.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = chain.Prune(ctx, appTx, req, time.Now())
	_ = appTx.Rollback(ctx)
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
		t.Fatalf("prune as app role: %v, want permission denied", err)
	}

	if _, err := e.prune(t, req); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if n := e.count(t); n != 4 { // 3 recent + audit.prune
		t.Fatalf("rows after prune: %d", n)
	}
	var meta string
	if err := e.app.QueryRow(ctx, `SELECT metadata::text FROM audit_events WHERE action = 'audit.prune'`).Scan(&meta); err != nil ||
		!strings.Contains(meta, seg.SHA256) {
		t.Fatalf("audit.prune event: %q %v", meta, err)
	}

	// Verification passes after prune + checkpoint, with anchors before, at and after the checkpoint.
	anchors := []chain.Anchor{{Seq: 3, Hash: strings.Repeat("ab", 32)}, {Seq: 5, Hash: hex.EncodeToString(anchor5)}, preHead}
	res := mustVerify(t, e.app, anchors)
	if !res.OK || res.CheckpointSeq != 5 || res.FromSeq != 6 || res.Checked != 4 || res.AnchorsChecked != 2 {
		t.Fatalf("verify after prune: %+v", res)
	}
	e.insert(t, 2)
	if res := mustVerify(t, e.app, anchors); !res.OK || res.HeadSeq != 11 {
		t.Fatalf("verify after new inserts: %+v", res)
	}

	// A forged checkpoint hash no longer links to the remaining rows.
	if _, err := e.owner.Exec(ctx, `ALTER TABLE audit_checkpoints DISABLE TRIGGER audit_checkpoints_no_update_delete;
		UPDATE audit_checkpoints SET last_hash = decode(repeat('11', 32), 'hex');
		ALTER TABLE audit_checkpoints ENABLE TRIGGER audit_checkpoints_no_update_delete`); err != nil {
		t.Fatal(err)
	}
	if res := mustVerify(t, e.app, nil); res.OK || res.BrokenSeq != 6 {
		t.Fatalf("forged checkpoint not detected: %+v", res)
	}
}

func TestAuditPruneRefusesBrokenChain(t *testing.T) {
	e := newChainEnv(t)
	e.insertAged(t, 3, 400*24*time.Hour)
	e.insert(t, 1)
	seg, err := chain.Export(context.Background(), e.app, 3, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	e.tamper(t, `UPDATE audit_events SET result = 'denied' WHERE chain_seq = 4`)
	_, err = e.prune(t, chain.PruneRequest{ThroughSeq: 3, ArchiveName: "a.jsonl.gz", ArchiveSHA256: seg.SHA256, RetentionDays: 365})
	if err == nil || !strings.Contains(err.Error(), "broken chain") {
		t.Fatalf("pruned a broken chain: %v", err)
	}
	if n := e.count(t); n != 4 {
		t.Fatalf("rows deleted: %d left", n)
	}
}

func TestAuditPruneEverythingThenContinue(t *testing.T) {
	ctx := context.Background()
	e := newChainEnv(t)
	e.insertAged(t, 4, 400*24*time.Hour)
	plan, err := chain.PlanPrune(ctx, e.app, 365, time.Now())
	if err != nil || plan.ThroughSeq != 4 {
		t.Fatalf("plan: %+v %v", plan, err)
	}
	seg, _ := chain.Export(ctx, e.app, 4, &bytes.Buffer{})
	if _, err := e.prune(t, chain.PruneRequest{ThroughSeq: 4, ArchiveName: "all.jsonl.gz", ArchiveSHA256: seg.SHA256, RetentionDays: 365}); err != nil {
		t.Fatal(err)
	}
	// Second round: only the audit.prune event remains and it is recent → nothing to prune.
	if p, _ := chain.PlanPrune(ctx, e.app, 365, time.Now()); p.Rows != 0 {
		t.Fatalf("second plan: %+v", p)
	}
	e.insert(t, 1)
	if res := mustVerify(t, e.app, nil); !res.OK || res.FromSeq != 5 || res.HeadSeq != 6 {
		t.Fatalf("verify: %+v", res)
	}
}
