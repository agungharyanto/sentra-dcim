// Package chain verifies the tamper-evident audit log (migration 000007, D-073) independently of the database: it
// rebuilds the canonical form of every row from its raw columns, recomputes
//
//	hash = sha256(prev_hash || canonical(row))
//
// along chain_seq from the latest pruning checkpoint, and checks external anchors (chain heads written into
// backups), so that even the database owner rewriting the whole chain is detected.
package chain

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// DB is what verification needs (pgxpool.Pool, pgx.Conn and pgx.Tx satisfy it). SELECT privilege suffices.
type DB interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Anchor is a chain head recorded outside the database (backup manifest).
type Anchor struct {
	Seq  int64  `json:"seq"`
	Hash string `json:"hash"` // hex
	At   string `json:"at,omitempty"`
}

// Result of a verification run.
type Result struct {
	OK             bool      `json:"ok"`
	Checked        int64     `json:"checked"`          // rows recomputed
	FromSeq        int64     `json:"from_seq"`         // first chain_seq checked (after the checkpoint)
	HeadSeq        int64     `json:"head_seq"`         // last chain_seq checked
	HeadHash       string    `json:"head_hash"`        // hex
	CheckpointSeq  int64     `json:"checkpoint_seq"`   // through_seq of the latest pruning checkpoint (0 = none)
	AnchorsChecked int       `json:"anchors_checked"`  //
	BrokenSeq      int64     `json:"broken_seq"`       // first bad chain_seq (0 = none)
	Reason         string    `json:"reason,omitempty"` // why it failed (never row contents)
	VerifiedAt     time.Time `json:"verified_at"`
}

var zero = make([]byte, 32)

const rowsSQL = `
SELECT chain_seq, id, to_char(occurred_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.US"Z"'), actor_type,
       coalesce(actor_id::text, ''), actor_username, action, target_type, target_id, result, coalesce(host(ip), ''),
       user_agent, request_id, metadata::text, prev_hash, hash
FROM audit_events WHERE chain_seq > $1 ORDER BY chain_seq`

// field appends <octet length>:<value> (same as SQL audit_field).
func field(b *bytes.Buffer, v string) {
	b.WriteString(strconv.Itoa(len(v)))
	b.WriteByte(':')
	b.WriteString(v)
}

// Canonical builds the canonical form from the textual column values in the fixed order of audit_canonical().
func Canonical(cols [14]string) []byte {
	var b bytes.Buffer
	for _, c := range cols {
		field(&b, c)
	}
	return b.Bytes()
}

// Hash returns sha256(prev || canonical).
func Hash(prev, canonical []byte) []byte {
	h := sha256.New()
	h.Write(prev)
	h.Write(canonical)
	return h.Sum(nil)
}

// Verify walks the chain. A database error is returned as error; a broken chain is a Result with OK=false.
func Verify(ctx context.Context, db DB, anchors []Anchor, now time.Time) (Result, error) {
	res := Result{VerifiedAt: now.UTC()}
	prev := zero
	var ckSeq int64
	var ckHash []byte
	err := db.QueryRow(ctx, `SELECT through_seq, last_hash FROM audit_checkpoints ORDER BY through_seq DESC LIMIT 1`).Scan(&ckSeq, &ckHash)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
	case err != nil:
		return res, fmt.Errorf("read checkpoint: %w", err)
	default:
		res.CheckpointSeq, prev = ckSeq, ckHash
	}
	byAnchor := map[int64][]byte{}
	for _, a := range anchors {
		h, err := hex.DecodeString(a.Hash)
		if err != nil || len(h) != 32 || a.Seq <= 0 {
			return res, fmt.Errorf("invalid anchor %+v", a)
		}
		if a.Seq <= ckSeq {
			if a.Seq == ckSeq {
				res.AnchorsChecked++
				if !bytes.Equal(h, ckHash) {
					return fail(res, ckSeq, "checkpoint hash differs from an anchor in a backup"), nil
				}
			}
			continue // pruned: covered by the archive
		}
		if prev, ok := byAnchor[a.Seq]; ok && !bytes.Equal(prev, h) {
			return fail(res, a.Seq, "two backups anchor different hashes for the same row"), nil
		}
		byAnchor[a.Seq] = h
	}

	rows, err := db.Query(ctx, rowsSQL, ckSeq)
	if err != nil {
		return res, fmt.Errorf("read audit rows: %w", err)
	}
	defer rows.Close()
	expect := ckSeq + 1
	for rows.Next() {
		var seq, id int64
		var cols [14]string
		var prevHash, hash []byte
		if err := rows.Scan(&seq, &id, &cols[2], &cols[3], &cols[4], &cols[5], &cols[6], &cols[7], &cols[8], &cols[9],
			&cols[10], &cols[11], &cols[12], &cols[13], &prevHash, &hash); err != nil {
			return res, fmt.Errorf("scan audit row: %w", err)
		}
		cols[0], cols[1] = strconv.FormatInt(seq, 10), strconv.FormatInt(id, 10)
		if res.Checked == 0 {
			res.FromSeq = seq
		}
		switch {
		case seq != expect:
			return fail(res, expect, fmt.Sprintf("row with chain_seq %d is missing", expect)), nil
		case !bytes.Equal(prevHash, prev):
			return fail(res, seq, "prev_hash does not match the previous row"), nil
		case !bytes.Equal(hash, Hash(prev, Canonical(cols))):
			return fail(res, seq, "row content does not match its hash (modified)"), nil
		}
		if a, ok := byAnchor[seq]; ok {
			res.AnchorsChecked++
			if !bytes.Equal(a, hash) {
				return fail(res, seq, "hash differs from the anchor in a backup (chain rewritten)"), nil
			}
			delete(byAnchor, seq)
		}
		res.Checked++
		res.HeadSeq, prev = seq, hash
		expect++
	}
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("read audit rows: %w", err)
	}
	for seq := range byAnchor {
		return fail(res, seq, fmt.Sprintf("anchored row %d no longer exists (chain truncated)", seq)), nil
	}
	res.OK = true
	res.HeadHash = hex.EncodeToString(prev)
	if res.HeadSeq == 0 {
		res.HeadSeq = ckSeq
	}
	return res, nil
}

func fail(r Result, seq int64, reason string) Result {
	r.OK, r.BrokenSeq, r.Reason = false, seq, reason
	return r
}

// Head returns the current chain head (for anchors in backups).
func Head(ctx context.Context, db DB) (Anchor, error) {
	var a Anchor
	var h []byte
	err := db.QueryRow(ctx, `SELECT chain_seq, hash FROM audit_events ORDER BY chain_seq DESC LIMIT 1`).Scan(&a.Seq, &h)
	if errors.Is(err, pgx.ErrNoRows) {
		err = db.QueryRow(ctx, `SELECT through_seq, last_hash FROM audit_checkpoints ORDER BY through_seq DESC LIMIT 1`).Scan(&a.Seq, &h)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return Anchor{}, nil
	}
	if err != nil {
		return Anchor{}, err
	}
	a.Hash = hex.EncodeToString(h)
	return a, nil
}

// ParseAnchors reads anchors: one JSON object per line ({"seq":…,"hash":"…"}); blank lines and empty anchors ({},
// a backup taken before the chain had rows) are ignored.
func ParseAnchors(s string) ([]Anchor, error) {
	var out []Anchor
	for i, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var a Anchor
		if err := json.Unmarshal([]byte(line), &a); err != nil {
			return nil, fmt.Errorf("anchor line %d: %w", i+1, err)
		}
		if a.Seq == 0 && a.Hash == "" {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}
