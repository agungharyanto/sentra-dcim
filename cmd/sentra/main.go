// Command sentra is the Sentra DCIM backend (modular monolith).
//
// Usage:
//
//	sentra serve                     start the HTTP API server
//	sentra migrate up|down [n]|goto <v>|force <v>|version
//	sentra healthcheck               probe the local liveness endpoint (container healthcheck)
//	sentra user create-admin --username <name> [--display-name <name>]
//	                                 create an administrator; password from the terminal (asked twice) or one
//	                                 line on stdin; it must be changed at first login
//	sentra version                   print build metadata
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata" // embed tz database; distroless images may lack it

	apispec "github.com/agungharyanto/sentra-dcim/api"
	helpdocs "github.com/agungharyanto/sentra-dcim/docs/help"
	"github.com/agungharyanto/sentra-dcim/internal/api"
	"github.com/agungharyanto/sentra-dcim/internal/audit/chain"
	auditrepo "github.com/agungharyanto/sentra-dcim/internal/audit/repo"
	auditservice "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	authhandler "github.com/agungharyanto/sentra-dcim/internal/auth/handler"
	authrepo "github.com/agungharyanto/sentra-dcim/internal/auth/repo"
	authservice "github.com/agungharyanto/sentra-dcim/internal/auth/service"
	helphandler "github.com/agungharyanto/sentra-dcim/internal/help/handler"
	helpservice "github.com/agungharyanto/sentra-dcim/internal/help/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/buildinfo"
	"github.com/agungharyanto/sentra-dcim/internal/platform/cache"
	"github.com/agungharyanto/sentra-dcim/internal/platform/config"
	"github.com/agungharyanto/sentra-dcim/internal/platform/database"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	"github.com/agungharyanto/sentra-dcim/internal/platform/migrator"
	systemhandler "github.com/agungharyanto/sentra-dcim/internal/system/handler"
	systemrepo "github.com/agungharyanto/sentra-dcim/internal/system/repo"
	systemservice "github.com/agungharyanto/sentra-dcim/internal/system/service"
	uihandler "github.com/agungharyanto/sentra-dcim/internal/ui/handler"
	uiservice "github.com/agungharyanto/sentra-dcim/internal/ui/service"
	usershandler "github.com/agungharyanto/sentra-dcim/internal/users/handler"
	usersservice "github.com/agungharyanto/sentra-dcim/internal/users/service"
)

func main() {
	args := os.Args[1:]
	cmd := "serve"
	if len(args) > 0 {
		cmd, args = args[0], args[1:]
	}

	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "migrate":
		err = runMigrate(args)
	case "healthcheck":
		err = healthcheck()
	case "user":
		err = runUser(args)
	case "db":
		err = runDB(args)
	case "audit":
		err = runAudit(args)
	case "version":
		err = json.NewEncoder(os.Stdout).Encode(buildinfo.Get())
	default:
		err = fmt.Errorf("unknown command %q (want serve|migrate|healthcheck|user|db|audit|version)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func serve() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := logging.New(os.Stdout, cfg.LogLevel)
	slog.SetDefault(log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := database.Connect(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()

	rdb, err := cache.Connect(ctx, cfg.Redis)
	if err != nil {
		return err
	}
	defer func() { _ = rdb.Close() }()

	latest, err := migrator.Latest()
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	sys := systemservice.New(log, cfg.DisplayTZ,
		systemservice.FuncChecker{CheckName: "database", Fn: pool.Ping},
		systemservice.FuncChecker{CheckName: "redis", Fn: func(ctx context.Context) error { return rdb.Ping(ctx).Err() }},
		systemservice.MigrationChecker(systemrepo.NewStore(pool), latest),
	)

	ui := uiservice.New(cfg.DefaultLocale, config.SupportedLocales, cfg.DisplayTZ)

	// Help content is validated by `go test` (the blocking gate). Should an invalid catalog still ship, only the
	// help module is disabled (503); the rest of the API and the healthcheck keep running.
	helpCat, err := helpservice.Load(helpdocs.FS, config.SupportedLocales, helpMenus(), apispec.OpenAPI)
	if err != nil {
		log.Error("help content invalid, help module disabled", "error", err)
	}
	help := helphandler.New(helpCat, cfg.DefaultLocale, config.SupportedLocales)

	rec := auditservice.New(auditrepo.New(pool), log)
	auth, err := authservice.New(authrepo.New(pool), authservice.NewRedisLimiter(rdb, authservice.DefaultLimiterPrefix), rec, log, cfg.Auth)
	if err != nil {
		return fmt.Errorf("auth: %w", err)
	}
	authH := authhandler.New(auth, log, cfg.Auth.AllowedOrigins)
	go cleanupSessions(ctx, auth, log)
	go rec.RunThrottle(ctx) // summaries of repeated denials (authz.denied.repeated, security.csrf_denied.repeated)
	// Daily hash-chain verification of the audit log (D-073); failures are logged at error level (Alarm Engine v0.5).
	integrity := chain.NewMonitor(pool, log, nil, 24*time.Hour)
	go integrity.Run(ctx)

	rl := httpserver.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	go rl.Janitor(ctx)

	// Public listener (behind Caddy's public site) and internal listener (Caddy 127.0.0.1 only) share all
	// modules; they differ only in the unauthenticated allowlist (internal adds /ready for ops scripts). Everything
	// else requires a session and, per route, a permission (internal/api, default-deny; D-053, D-054).
	users := usershandler.New(usersservice.New(pool, rec, log, cfg.Auth), log)
	mods := api.Modules{System: systemhandler.New(sys), UI: uihandler.New(ui), Help: help, Auth: authH, Users: users}
	servers := []*http.Server{
		httpserver.NewServer(cfg.HTTPAddr, httpserver.NewRouter(log, rl, cfg.TrustedProxies, api.Mount(mods, false)), log),
		httpserver.NewServer(cfg.InternalHTTPAddr, httpserver.NewRouter(log, rl, cfg.TrustedProxies, api.Mount(mods, true)), log),
	}

	errCh := make(chan error, len(servers))
	for _, srv := range servers {
		go func() {
			log.Info("http server starting", "addr", srv.Addr, "version", buildinfo.Version, "env", cfg.Env)
			if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errCh <- fmt.Errorf("%s: %w", srv.Addr, err)
			}
		}()
	}

	var serveErr error
	select {
	case serveErr = <-errCh:
		log.Error("http server failed, shutting down", "error", serveErr)
	case <-ctx.Done():
		log.Info("shutdown signal received, draining", "timeout", cfg.ShutdownTimeout.String())
	}

	sys.SetDraining()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	var wg sync.WaitGroup
	shutdownErrs := make([]error, len(servers))
	for i, srv := range servers {
		wg.Go(func() {
			if err := srv.Shutdown(shutdownCtx); err != nil {
				shutdownErrs[i] = fmt.Errorf("graceful shutdown %s: %w", srv.Addr, err)
			}
		})
	}
	wg.Wait()
	if err := errors.Join(append([]error{serveErr}, shutdownErrs...)...); err != nil {
		return err
	}
	log.Info("server stopped cleanly")
	return nil
}

// cleanupSessions deletes stale sessions at start-up and then hourly.
func cleanupSessions(ctx context.Context, auth *authservice.Service, log *slog.Logger) {
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	for {
		if n, err := auth.CleanupStale(ctx); err != nil && ctx.Err() == nil {
			log.Warn("session cleanup failed", "error", err)
		} else if n > 0 {
			log.Info("stale sessions deleted", "count", n)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// helpMenus adapts the UI menu registry for help validation (menu keys, app routes, available menus).
func helpMenus() []helpservice.Menu {
	_, menus := uiservice.Registry()
	out := make([]helpservice.Menu, 0, len(menus))
	for _, m := range menus {
		out = append(out, helpservice.Menu{Key: m.Key, Path: m.Path, Available: m.Status == uiservice.StatusAvailable})
	}
	return out
}

func runMigrate(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: sentra migrate up|down [n]|goto <version>|force <version>|version")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log := logging.New(os.Stdout, cfg.LogLevel)
	m, err := migrator.New(cfg.DB.DSN(), log)
	if err != nil {
		return err
	}
	defer func() { _ = m.Close() }()

	intArg := func(def int) (int, error) {
		if len(args) < 2 {
			if def < 0 {
				return 0, fmt.Errorf("migrate %s: missing argument", args[0])
			}
			return def, nil
		}
		n, err := strconv.Atoi(args[1])
		if err != nil || n < 0 {
			return 0, fmt.Errorf("migrate %s: invalid number %q", args[0], args[1])
		}
		return n, nil
	}

	switch args[0] {
	case "up":
		err = m.Up()
	case "down":
		var n int
		if n, err = intArg(1); err == nil {
			err = m.Down(n)
		}
	case "goto":
		var v int
		if v, err = intArg(-1); err == nil {
			err = m.Goto(uint(v))
		}
	case "force":
		var v int
		if v, err = intArg(-1); err == nil {
			err = m.Force(v)
		}
	case "version":
	default:
		return fmt.Errorf("unknown migrate subcommand %q", args[0])
	}
	if err != nil {
		return err
	}

	v, dirty, ok, err := m.Version()
	if err != nil {
		return err
	}
	latest, _ := migrator.Latest()
	if !ok {
		fmt.Printf("version=none dirty=false latest=%d\n", latest)
		return nil
	}
	fmt.Printf("version=%d dirty=%t latest=%d\n", v, dirty, latest)
	return nil
}

func healthcheck() error {
	addr := os.Getenv("APP_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("APP_HTTP_ADDR: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	url := "http://" + net.JoinHostPort("127.0.0.1", port) + "/api/v1/health"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil) //nolint:gosec // loopback only
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req) //nolint:gosec // loopback only
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health status %d", resp.StatusCode)
	}
	return nil
}
