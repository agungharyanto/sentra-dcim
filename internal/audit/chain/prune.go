package chain

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
)

// MinRetentionDays is the shortest audit retention accepted (AUDIT_RETENTION_DAYS).
const MinRetentionDays = 90

// Plan is the segment a pruning run would remove: chain_seq in (FromSeq-1, ThroughSeq], i.e. the oldest rows up to
// the first row that is still inside the retention window. Pruning only ever removes a prefix of the chain.
type Plan struct {
	FromSeq    int64     `json:"from_seq"`    // first row to prune (checkpoint + 1)
	ThroughSeq int64     `json:"through_seq"` // last row to prune (== FromSeq-1: nothing to prune)
	Rows       int64     `json:"rows"`
	Cutoff     time.Time `json:"cutoff"` // rows older than this are outside retention
}

// PlanPrune computes the segment outside the retention window.
func PlanPrune(ctx context.Context, db DB, retentionDays int, now time.Time) (Plan, error) {
	if retentionDays < MinRetentionDays {
		return Plan{}, fmt.Errorf("audit retention %d days is below the minimum of %d", retentionDays, MinRetentionDays)
	}
	p := Plan{Cutoff: now.UTC().AddDate(0, 0, -retentionDays)}
	ck, err := checkpointSeq(ctx, db)
	if err != nil {
		return p, err
	}
	// Last row of the longest prefix whose rows are all older than the cutoff.
	err = db.QueryRow(ctx, `
		SELECT coalesce((SELECT min(chain_seq) FROM audit_events WHERE occurred_at >= $1) - 1,
		                (SELECT max(chain_seq) FROM audit_events), $2)`, p.Cutoff, ck).Scan(&p.ThroughSeq)
	if err != nil {
		return p, fmt.Errorf("plan prune: %w", err)
	}
	p.FromSeq = ck + 1
	if p.ThroughSeq < ck {
		p.ThroughSeq = ck
	}
	p.Rows = p.ThroughSeq - ck
	return p, nil
}

func checkpointSeq(ctx context.Context, db DB) (int64, error) {
	var ck int64
	if err := db.QueryRow(ctx, `SELECT coalesce(max(through_seq), 0) FROM audit_checkpoints`).Scan(&ck); err != nil {
		return 0, fmt.Errorf("read checkpoint: %w", err)
	}
	return ck, nil
}

// archiveRow is one line of an archive: every column needed to re-verify the segment offline.
type archiveRow struct {
	ChainSeq      int64  `json:"chain_seq"`
	ID            int64  `json:"id"`
	OccurredAt    string `json:"occurred_at"`
	ActorType     string `json:"actor_type"`
	ActorID       string `json:"actor_id"`
	ActorUsername string `json:"actor_username"`
	Action        string `json:"action"`
	TargetType    string `json:"target_type"`
	TargetID      string `json:"target_id"`
	Result        string `json:"result"`
	IP            string `json:"ip"`
	UserAgent     string `json:"user_agent"`
	RequestID     string `json:"request_id"`
	Metadata      string `json:"metadata"` // jsonb text exactly as hashed (not re-encoded)
	PrevHash      string `json:"prev_hash"`
	Hash          string `json:"hash"`
}

// Segment describes an exported archive.
type Segment struct {
	FromSeq, ThroughSeq int64
	Rows                int64
	OldestAt, NewestAt  string
	LastHash            []byte
	SHA256              string // of the compressed archive
}

// Export writes rows (checkpoint, through] as gzip-compressed JSON lines (a header line, then one row per line).
// The output is deterministic for the same rows, so Prune can re-create it and compare the checksum with the
// archive the operator stored before anything is deleted.
func Export(ctx context.Context, db DB, through int64, w io.Writer) (Segment, error) {
	ck, err := checkpointSeq(ctx, db)
	if err != nil {
		return Segment{}, err
	}
	seg := Segment{FromSeq: ck + 1, ThroughSeq: through}
	if through <= ck {
		return seg, fmt.Errorf("nothing to export: through_seq %d is not after checkpoint %d", through, ck)
	}
	sum := sha256.New()
	zw, err := gzip.NewWriterLevel(io.MultiWriter(w, sum), gzip.BestCompression) // zero ModTime/Name: deterministic
	if err != nil {
		return seg, err
	}
	enc := json.NewEncoder(zw)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(map[string]any{"sentra_audit_archive": 1, "from_seq": seg.FromSeq, "through_seq": through}); err != nil {
		return seg, err
	}
	rows, err := db.Query(ctx, rowsSQL+` LIMIT $2`, ck, through-ck)
	if err != nil {
		return seg, fmt.Errorf("read audit rows: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var r archiveRow
		var prev, hash []byte
		if err := rows.Scan(&r.ChainSeq, &r.ID, &r.OccurredAt, &r.ActorType, &r.ActorID, &r.ActorUsername, &r.Action,
			&r.TargetType, &r.TargetID, &r.Result, &r.IP, &r.UserAgent, &r.RequestID, &r.Metadata, &prev, &hash); err != nil {
			return seg, fmt.Errorf("scan audit row: %w", err)
		}
		if r.ChainSeq != seg.FromSeq+seg.Rows {
			return seg, fmt.Errorf("chain_seq %d missing in the segment", seg.FromSeq+seg.Rows)
		}
		r.PrevHash, r.Hash = hex.EncodeToString(prev), hex.EncodeToString(hash)
		if err := enc.Encode(r); err != nil {
			return seg, err
		}
		if seg.Rows == 0 {
			seg.OldestAt = r.OccurredAt
		}
		seg.Rows++
		seg.NewestAt, seg.LastHash = r.OccurredAt, hash
	}
	if err := rows.Err(); err != nil {
		return seg, fmt.Errorf("read audit rows: %w", err)
	}
	if seg.Rows != through-ck {
		return seg, fmt.Errorf("segment has %d rows, want %d", seg.Rows, through-ck)
	}
	if err := zw.Close(); err != nil {
		return seg, err
	}
	seg.SHA256 = hex.EncodeToString(sum.Sum(nil))
	return seg, nil
}

var archiveNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,199}$`)

// PruneRequest is what the operator confirms after storing the archive.
type PruneRequest struct {
	ThroughSeq    int64
	ArchiveName   string
	ArchiveSHA256 string
	RetentionDays int
}

// ErrArchiveMismatch means the stored archive is not exactly the segment that would be deleted.
var ErrArchiveMismatch = errors.New("archive checksum does not match the segment to prune")

// Prune removes the segment (checkpoint, ThroughSeq] inside tx, which must run as the table owner (migration role).
// It refuses when the chain is broken, when the segment reaches into the retention window, or when the archive
// checksum differs from a fresh export; it leaves a checkpoint with the segment's last hash, records an audit.prune
// event and re-verifies before the caller commits.
func Prune(ctx context.Context, tx pgx.Tx, req PruneRequest, now time.Time) (Segment, error) {
	if !archiveNameRe.MatchString(req.ArchiveName) {
		return Segment{}, errors.New("invalid archive name")
	}
	if _, err := hex.DecodeString(req.ArchiveSHA256); err != nil || len(req.ArchiveSHA256) != 64 {
		return Segment{}, errors.New("invalid archive sha256 (64 hex)")
	}
	// Blocks concurrent inserts (and their chain trigger) but not reads, for the whole run.
	if _, err := tx.Exec(ctx, `LOCK TABLE audit_events IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return Segment{}, fmt.Errorf("lock audit_events: %w", err)
	}
	plan, err := PlanPrune(ctx, tx, req.RetentionDays, now)
	if err != nil {
		return Segment{}, err
	}
	if req.ThroughSeq > plan.ThroughSeq {
		return Segment{}, fmt.Errorf("through_seq %d reaches into the retention window (max %d)", req.ThroughSeq, plan.ThroughSeq)
	}
	res, err := Verify(ctx, tx, nil, now)
	if err != nil {
		return Segment{}, err
	}
	if !res.OK {
		return Segment{}, fmt.Errorf("refusing to prune a broken chain (chain_seq %d: %s)", res.BrokenSeq, res.Reason)
	}
	seg, err := Export(ctx, tx, req.ThroughSeq, io.Discard)
	if err != nil {
		return seg, err
	}
	if seg.SHA256 != req.ArchiveSHA256 {
		return seg, ErrArchiveMismatch
	}
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO audit_checkpoints (through_seq, last_hash, rows_pruned, oldest_at, newest_at, archive_name, archive_sha256)
		  VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			[]any{seg.ThroughSeq, seg.LastHash, seg.Rows, seg.OldestAt, seg.NewestAt, req.ArchiveName, seg.SHA256}},
		{`ALTER TABLE audit_events DISABLE TRIGGER audit_events_no_update_delete`, nil},
		{`DELETE FROM audit_events WHERE chain_seq <= $1`, []any{seg.ThroughSeq}},
		{`ALTER TABLE audit_events ENABLE TRIGGER audit_events_no_update_delete`, nil},
		{`INSERT INTO audit_events (actor_type, actor_username, action, target_type, target_id, result, metadata)
		  VALUES ('system', 'sentra-ops', 'audit.prune', 'audit_segment', $1, 'success', $2)`,
			[]any{strconv.FormatInt(seg.FromSeq, 10) + "-" + strconv.FormatInt(seg.ThroughSeq, 10), map[string]any{
				"rows": seg.Rows, "through_seq": seg.ThroughSeq, "archive_name": req.ArchiveName,
				"archive_sha256": seg.SHA256, "retention_days": req.RetentionDays, "oldest_at": seg.OldestAt,
				"newest_at": seg.NewestAt}}},
	} {
		if _, err := tx.Exec(ctx, q.sql, q.args...); err != nil {
			return seg, fmt.Errorf("prune: %w", err)
		}
	}
	after, err := Verify(ctx, tx, nil, now)
	if err != nil {
		return seg, err
	}
	if !after.OK || after.CheckpointSeq != seg.ThroughSeq {
		return seg, fmt.Errorf("chain does not verify after pruning (chain_seq %d: %s)", after.BrokenSeq, after.Reason)
	}
	return seg, nil
}

// ArchiveCheck is the result of verifying an archive file offline.
type ArchiveCheck struct {
	FromSeq    int64  `json:"from_seq"`
	ThroughSeq int64  `json:"through_seq"`
	Rows       int64  `json:"rows"`
	FirstPrev  string `json:"first_prev_hash"` // must equal the previous checkpoint's last_hash (zeros for the first)
	LastHash   string `json:"last_hash"`       // must equal this segment's checkpoint last_hash
}

// VerifyArchive recomputes the hash chain inside an archive written by Export (gzip JSON lines). The caller compares
// FirstPrev/LastHash with audit_checkpoints to tie the archive to the live chain.
func VerifyArchive(r io.Reader) (ArchiveCheck, error) {
	var c ArchiveCheck
	zr, err := gzip.NewReader(r)
	if err != nil {
		return c, fmt.Errorf("archive: %w", err)
	}
	dec := json.NewDecoder(zr)
	var head struct {
		Version    int   `json:"sentra_audit_archive"`
		FromSeq    int64 `json:"from_seq"`
		ThroughSeq int64 `json:"through_seq"`
	}
	if err := dec.Decode(&head); err != nil {
		return c, fmt.Errorf("archive header: %w", err)
	}
	if head.Version != 1 {
		return c, fmt.Errorf("archive header: unsupported version %d", head.Version)
	}
	c.FromSeq, c.ThroughSeq = head.FromSeq, head.ThroughSeq
	var prev []byte
	for {
		var a archiveRow
		if err := dec.Decode(&a); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return c, fmt.Errorf("archive row %d: %w", c.Rows+1, err)
		}
		want := c.FromSeq + c.Rows
		if a.ChainSeq != want {
			return c, fmt.Errorf("archive: chain_seq %d where %d expected", a.ChainSeq, want)
		}
		rowPrev, err1 := hex.DecodeString(a.PrevHash)
		hash, err2 := hex.DecodeString(a.Hash)
		if err1 != nil || err2 != nil {
			return c, fmt.Errorf("archive: chain_seq %d has invalid hashes", a.ChainSeq)
		}
		if c.Rows == 0 {
			c.FirstPrev = a.PrevHash
		} else if !bytes.Equal(rowPrev, prev) {
			return c, fmt.Errorf("archive: chain_seq %d prev_hash does not match the previous row", a.ChainSeq)
		}
		cols := [14]string{strconv.FormatInt(a.ChainSeq, 10), strconv.FormatInt(a.ID, 10), a.OccurredAt, a.ActorType,
			a.ActorID, a.ActorUsername, a.Action, a.TargetType, a.TargetID, a.Result, a.IP, a.UserAgent, a.RequestID,
			a.Metadata}
		if !bytes.Equal(hash, Hash(rowPrev, Canonical(cols))) {
			return c, fmt.Errorf("archive: chain_seq %d content does not match its hash", a.ChainSeq)
		}
		prev = hash
		c.Rows++
	}
	if c.Rows == 0 || c.FromSeq+c.Rows-1 != c.ThroughSeq {
		return c, fmt.Errorf("archive holds %d rows, header says %d..%d", c.Rows, c.FromSeq, c.ThroughSeq)
	}
	c.LastHash = hex.EncodeToString(prev)
	return c, nil
}
