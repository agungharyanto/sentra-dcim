// Command sentra is the Sentra DCIM backend (modular monolith).
//
// Usage:
//
//	sentra serve                     start the HTTP API server
//	sentra migrate up|down [n]|goto <v>|force <v>|version
//	sentra healthcheck               probe the local liveness endpoint (container healthcheck)
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

	"github.com/go-chi/chi/v5"

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
	case "version":
		err = json.NewEncoder(os.Stdout).Encode(buildinfo.Get())
	default:
		err = fmt.Errorf("unknown command %q (want serve|migrate|healthcheck|version)", cmd)
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

	rl := httpserver.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
	go rl.Janitor(ctx)

	// Public listener (behind Caddy's public site) and internal listener (Caddy 127.0.0.1 only) share all
	// dependencies; they differ in how much operational detail they return.
	servers := []*http.Server{
		httpserver.NewServer(cfg.HTTPAddr, httpserver.NewRouter(log, rl, cfg.TrustedProxies, func(r chi.Router) {
			systemhandler.New(sys).Routes(r)
		}), log),
		httpserver.NewServer(cfg.InternalHTTPAddr, httpserver.NewRouter(log, rl, cfg.TrustedProxies, func(r chi.Router) {
			systemhandler.NewInternal(sys).Routes(r)
		}), log),
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
