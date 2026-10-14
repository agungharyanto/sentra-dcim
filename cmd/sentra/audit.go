package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/agungharyanto/sentra-dcim/internal/audit/chain"
	"github.com/agungharyanto/sentra-dcim/internal/platform/config"
	"github.com/agungharyanto/sentra-dcim/internal/platform/database"
)

const auditUsage = `usage: sentra audit verify [--anchors -]   verify the hash chain (anchors: JSON lines on stdin)
       sentra audit head                    print the chain head as an anchor (JSON)
       sentra audit prune-plan              print the segment outside AUDIT_RETENTION_DAYS (JSON)
       sentra audit export --through N      write that segment as gzip JSON lines to stdout
       sentra audit verify-archive < FILE   recompute the chain inside an archive (offline, no database)
       sentra audit prune --through N --archive-name NAME --archive-sha256 HEX
                                            delete the archived segment, leave a checkpoint (owner role only)`

// runAudit implements `sentra audit …`. verify/head/prune-plan/export need SELECT only (application role); prune
// needs the table owner and runs from the ops container (ops/audit-prune.sh).
func runAudit(args []string) error {
	if len(args) == 0 {
		return errors.New(auditUsage)
	}
	if len(args) == 1 && args[0] == "verify-archive" {
		c, err := chain.VerifyArchive(bufio.NewReader(os.Stdin))
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(c)
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	pool, err := database.Connect(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()
	out := json.NewEncoder(os.Stdout)

	switch {
	case args[0] == "head" && len(args) == 1:
		a, err := chain.Head(ctx, pool)
		if err != nil {
			return err
		}
		a.At = time.Now().UTC().Format(time.RFC3339)
		return out.Encode(a)
	case args[0] == "prune-plan" && len(args) == 1:
		p, err := chain.PlanPrune(ctx, pool, cfg.AuditRetentionDays, time.Now())
		if err != nil {
			return err
		}
		return out.Encode(p)
	case args[0] == "export":
		fs := flag.NewFlagSet("audit export", flag.ContinueOnError)
		through := fs.Int64("through", 0, "last chain_seq of the segment")
		if err := fs.Parse(args[1:]); err != nil || *through <= 0 || fs.NArg() != 0 {
			return errors.New(auditUsage)
		}
		w := bufio.NewWriter(os.Stdout)
		seg, err := chain.Export(ctx, pool, *through, w)
		if err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "exported chain_seq %d..%d (%d rows), sha256 %s\n", seg.FromSeq, seg.ThroughSeq, seg.Rows, seg.SHA256)
		return nil
	case args[0] == "prune":
		fs := flag.NewFlagSet("audit prune", flag.ContinueOnError)
		req := chain.PruneRequest{RetentionDays: cfg.AuditRetentionDays}
		fs.Int64Var(&req.ThroughSeq, "through", 0, "last chain_seq to prune")
		fs.StringVar(&req.ArchiveName, "archive-name", "", "file name of the stored archive")
		fs.StringVar(&req.ArchiveSHA256, "archive-sha256", "", "sha256 of the stored archive")
		if err := fs.Parse(args[1:]); err != nil || req.ThroughSeq <= 0 || fs.NArg() != 0 {
			return errors.New(auditUsage)
		}
		tx, err := pool.Begin(ctx)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback(ctx) }()
		seg, err := chain.Prune(ctx, tx, req, time.Now())
		if err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return out.Encode(map[string]any{"pruned_rows": seg.Rows, "from_seq": seg.FromSeq, "through_seq": seg.ThroughSeq,
			"archive_name": req.ArchiveName, "archive_sha256": seg.SHA256})
	case args[0] == "verify" && (len(args) == 1 || (len(args) == 3 && args[1] == "--anchors" && args[2] == "-")):
		var anchors []chain.Anchor
		if len(args) == 3 {
			raw, err := io.ReadAll(io.LimitReader(os.Stdin, 16<<20))
			if err != nil {
				return err
			}
			if anchors, err = chain.ParseAnchors(string(raw)); err != nil {
				return err
			}
		}
		res, err := chain.Verify(ctx, pool, anchors, time.Now())
		if err != nil {
			return err
		}
		_ = out.Encode(res)
		if !res.OK {
			return fmt.Errorf("audit chain BROKEN at chain_seq %d: %s", res.BrokenSeq, res.Reason)
		}
		return nil
	}
	return errors.New(auditUsage)
}
