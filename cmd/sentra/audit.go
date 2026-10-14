package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/agungharyanto/sentra-dcim/internal/audit/chain"
	"github.com/agungharyanto/sentra-dcim/internal/platform/config"
	"github.com/agungharyanto/sentra-dcim/internal/platform/database"
)

const auditUsage = `usage: sentra audit verify [--anchors -]   verify the hash chain (anchors: JSON lines on stdin)
       sentra audit head                    print the chain head as an anchor (JSON)`

// runAudit implements `sentra audit …` (SELECT only: works with the application role).
func runAudit(args []string) error {
	if len(args) == 0 {
		return errors.New(auditUsage)
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
