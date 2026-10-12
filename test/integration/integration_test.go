//go:build integration

// Package integration runs tests against real PostgreSQL/TimescaleDB and Redis.
// Each run uses a throwaway database so the application database is never touched.
//
//	make up && make test-integration
package integration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/agungharyanto/sentra-dcim/internal/platform/cache"
	"github.com/agungharyanto/sentra-dcim/internal/platform/config"
	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	"github.com/agungharyanto/sentra-dcim/internal/platform/migrator"
	"github.com/agungharyanto/sentra-dcim/internal/system/repo"
	"github.com/agungharyanto/sentra-dcim/internal/system/service"
)

func adminURL(t *testing.T) *url.URL {
	t.Helper()
	raw := os.Getenv("SENTRA_IT_DATABASE_URL")
	if raw == "" {
		t.Skip("SENTRA_IT_DATABASE_URL not set")
	}
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

// tempDB creates an isolated database and returns its URL; it is dropped on cleanup.
func tempDB(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	admin := adminURL(t)
	conn, err := pgx.Connect(ctx, admin.String())
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	name := "sentra_it_" + hex.EncodeToString(b)
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatalf("create db: %v", err)
	}
	t.Cleanup(func() {
		_, _ = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		_ = conn.Close(ctx)
	})
	u := *admin
	u.Path = "/" + name
	return u.String()
}

func tableExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var ok bool
	if err := pool.QueryRow(context.Background(), "SELECT to_regclass($1) IS NOT NULL", name).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

func TestMigrationsUpDownUp(t *testing.T) {
	ctx := context.Background()
	dsn := tempDB(t)
	log := logging.Discard()

	m, err := migrator.New(dsn, log)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = m.Close() }()
	latest, _ := migrator.Latest()

	if err := m.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}
	if v, dirty, ok, _ := m.Version(); !ok || dirty || v != latest {
		t.Fatalf("after up: v=%d dirty=%t ok=%t", v, dirty, ok)
	}
	if err := m.Up(); err != nil { // idempotent
		t.Fatalf("second up: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	// sqlc-generated repo + readiness migration check.
	store := repo.NewStore(pool)
	if err := service.MigrationChecker(store, latest).Check(ctx); err != nil {
		t.Fatalf("migration checker: %v", err)
	}
	info, err := repo.New(pool).GetSystemInfo(ctx, "installation.initialized_at")
	if err != nil || info.Value == "" {
		t.Fatalf("system_info: %+v %v", info, err)
	}
	var ext string
	if err := pool.QueryRow(ctx, "SELECT extversion FROM pg_extension WHERE extname='timescaledb'").Scan(&ext); err != nil {
		t.Fatalf("timescaledb extension: %v", err)
	}
	t.Logf("timescaledb %s", ext)

	// Parameterised input validation at the DB layer: invalid keys are rejected.
	if _, err := pool.Exec(ctx, "INSERT INTO system_info(key,value) VALUES ($1,$2)", "Bad Key; DROP", "x"); err == nil {
		t.Fatal("expected CHECK constraint violation")
	}

	if err := m.Goto(0); err != nil {
		t.Fatalf("down to 0: %v", err)
	}
	if tableExists(t, pool, "system_info") {
		t.Fatal("system_info should be dropped after down")
	}
	if err := service.MigrationChecker(store, latest).Check(ctx); err == nil {
		t.Fatal("migration checker should fail when schema is behind")
	}

	if err := m.Up(); err != nil {
		t.Fatalf("re-up: %v", err)
	}
	if !tableExists(t, pool, "system_info") {
		t.Fatal("system_info missing after re-up")
	}
}

func TestSessionTimezoneUTC(t *testing.T) {
	ctx := context.Background()
	dsn := tempDB(t)
	pool, err := pgxpool.New(ctx, dsn+"&timezone=UTC")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	var tz string
	if err := pool.QueryRow(ctx, "SHOW timezone").Scan(&tz); err != nil || tz != "UTC" {
		t.Fatalf("timezone = %q, %v", tz, err)
	}
}

func TestRedis(t *testing.T) {
	addr := os.Getenv("SENTRA_IT_REDIS_ADDR")
	if addr == "" {
		t.Skip("SENTRA_IT_REDIS_ADDR not set")
	}
	ctx := context.Background()

	if _, err := cache.Connect(ctx, config.Redis{Addr: addr, Password: "wrong-password"}); err == nil {
		t.Fatal("expected auth failure with wrong password")
	}
	c, err := cache.Connect(ctx, config.Redis{Addr: addr, Password: os.Getenv("SENTRA_IT_REDIS_PASSWORD")})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	key := "sentra:it:" + time.Now().Format(time.RFC3339Nano)
	if err := c.Set(ctx, key, "1", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if v, err := c.Get(ctx, key).Result(); err != nil || v != "1" {
		t.Fatalf("get: %q %v", v, err)
	}
	_ = c.Del(ctx, key)
	if _, err := c.Get(ctx, key).Result(); !errors.Is(err, redis.Nil) {
		t.Fatalf("expected redis.Nil, got %v", err)
	}
}
