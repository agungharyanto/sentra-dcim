package main

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	allowlistservice "github.com/agungharyanto/sentra-dcim/internal/allowlist/service"
	auditrepo "github.com/agungharyanto/sentra-dcim/internal/audit/repo"
	auditservice "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/config"
	"github.com/agungharyanto/sentra-dcim/internal/platform/database"
	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	settingsservice "github.com/agungharyanto/sentra-dcim/internal/settings/service"
)

const allowlistUsage = `usage: sentra allowlist list
       sentra allowlist add CIDR [LABEL...]     add an IP address or network
       sentra allowlist remove CIDR             remove a network (exactly as listed)
       sentra allowlist mode off|monitor        emergency exit: stop blocking (takes effect within 5 s)
       sentra allowlist mode enforce --force    enforce WITHOUT lockout guard and auto-rollback (tests only)`

// runAllowlist is the server-side emergency door of Allowed IP (v0.3.6, D-078): over SSH or the serial console,
// `docker compose exec -T app /sentra allowlist mode off`. Changes are audited as system "cli"; the running app
// picks them up within a few seconds.
func runAllowlist(args []string) error {
	if len(args) == 0 {
		return errors.New(allowlistUsage)
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := database.Connect(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()
	log := logging.New(os.Stderr, cfg.LogLevel)
	reg, err := settingsservice.NewRegistry(settingsservice.Defaults{Language: cfg.DefaultLocale, Timezone: cfg.DisplayTZ.String()})
	if err != nil {
		return err
	}
	rec := auditservice.New(auditrepo.New(pool), log)
	set := settingsservice.New(pool, reg, rec, log)
	svc := allowlistservice.New(pool, set, rec, log)
	meta := auditservice.Meta{UserAgent: "sentra-cli"}

	switch {
	case args[0] == "list" && len(args) == 1:
		st, err := svc.State(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("mode: %s\n", st.Mode)
		if st.PendingUntil != nil {
			fmt.Printf("enforce waiting for confirmation until %s (then back to monitor)\n", st.PendingUntil.Format(time.RFC3339))
		}
		tw := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		_, _ = fmt.Fprintln(tw, "CIDR\tLABEL\tADDED (UTC)")
		for _, e := range st.Entries {
			_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", e.CIDR, e.Label, e.CreatedAt.Format("2006-01-02 15:04"))
		}
		_ = tw.Flush()
		fmt.Printf("%d network(s); always public: /api/v1/health, /.well-known/acme-challenge/*\n", len(st.Entries))
		return nil
	case args[0] == "add" && len(args) >= 2:
		e, err := svc.Add(ctx, args[1], strings.Join(args[2:], " "), meta)
		if err != nil {
			return cliErr(err)
		}
		fmt.Printf("added %s\n", e.CIDR)
		return nil
	case args[0] == "remove" && len(args) == 2:
		cidr, err := svc.Remove(ctx, nil, args[1], netip.Addr{}, meta)
		if err != nil {
			return cliErr(err)
		}
		fmt.Printf("removed %s\n", cidr)
		return nil
	case args[0] == "mode" && len(args) >= 2:
		mode, force := args[1], len(args) == 3 && args[2] == "--force"
		if len(args) > 3 || (len(args) == 3 && !force) {
			return errors.New(allowlistUsage)
		}
		if mode == allowlistservice.ModeEnforce && !force {
			return errors.New("enforce from the CLI has no lockout guard: switch to enforce in Settings › Allowed IP " +
				"(guard + auto-rollback), or add --force if you really mean it")
		}
		st, err := svc.SetMode(ctx, mode, allowlistservice.ModeOptions{Force: force}, meta)
		if err != nil {
			return cliErr(err)
		}
		fmt.Printf("mode: %s (the app applies it within %s)\n", st.Mode, allowlistservice.ReloadInterval)
		return nil
	}
	return errors.New(allowlistUsage)
}

func cliErr(err error) error {
	var ve *allowlistservice.ValidationError
	if errors.As(err, &ve) {
		return fmt.Errorf("%s: %s", ve.Field, ve.Reason)
	}
	return err
}
