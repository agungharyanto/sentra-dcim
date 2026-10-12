// Package database manages the PostgreSQL connection pool.
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/agungharyanto/sentra-dcim/internal/platform/config"
)

// Connect creates a pgx pool and verifies connectivity. Sessions always use UTC.
func Connect(ctx context.Context, cfg config.DB) (*pgxpool.Pool, error) {
	pc, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse db config: %w", err)
	}
	pc.MaxConns = cfg.MaxConns
	pc.MaxConnIdleTime = 5 * time.Minute
	pc.HealthCheckPeriod = 30 * time.Second
	pc.ConnConfig.RuntimeParams["timezone"] = "UTC"
	pc.ConnConfig.RuntimeParams["application_name"] = "sentra"

	pool, err := pgxpool.NewWithConfig(ctx, pc)
	if err != nil {
		return nil, fmt.Errorf("create db pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping db: %w", err)
	}
	return pool, nil
}
