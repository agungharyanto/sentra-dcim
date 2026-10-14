// Package service implements system status logic: liveness, readiness and version.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agungharyanto/sentra-dcim/internal/platform/buildinfo"
)

// Check status values.
const (
	StatusOK          = "ok"
	StatusReady       = "ready"
	StatusNotReady    = "not_ready"
	StatusUnavailable = "unavailable"
)

// Checker is a dependency that must be healthy for the service to be ready.
type Checker interface {
	Name() string
	Check(ctx context.Context) error
}

// CheckResult is the outcome of a single dependency check.
type CheckResult struct {
	Status    string `json:"status"`
	LatencyMS int64  `json:"latency_ms"`
}

// Readiness is the aggregated readiness report.
type Readiness struct {
	Status    string                 `json:"status"`
	Checks    map[string]CheckResult `json:"checks"`
	CheckedAt time.Time              `json:"checked_at"`
}

// PublicVersion is the only build information exposed to unauthenticated public clients.
type PublicVersion struct {
	Version string `json:"version"`
}

// Version describes the running build and server clock (internal listener only).
type Version struct {
	buildinfo.Info
	ServerTimeUTC     time.Time `json:"server_time_utc"`
	DisplayTimezone   string    `json:"display_timezone"`
	ServerTimeDisplay string    `json:"server_time_display"`
}

// Service aggregates system status.
type Service struct {
	checkers     []Checker
	checkTimeout time.Duration
	tz           *time.Location
	location     func() *time.Location // system display time zone from Settings – Regional (v0.3.5); nil = tz
	now          func() time.Time
	log          *slog.Logger
	draining     atomic.Bool
}

// New builds a Service. tz is the display timezone (e.g. Asia/Jakarta).
func New(log *slog.Logger, tz *time.Location, checkers ...Checker) *Service {
	return &Service{
		checkers:     checkers,
		checkTimeout: 2 * time.Second,
		tz:           tz,
		now:          time.Now,
		log:          log,
	}
}

// SetLocation makes Version follow the system time zone setting (Settings – Regional) instead of DISPLAY_TIMEZONE.
func (s *Service) SetLocation(f func() *time.Location) { s.location = f }

// SetDraining marks the service as shutting down; readiness then reports not_ready.
func (s *Service) SetDraining() { s.draining.Store(true) }

// Readiness runs all checks concurrently. Detailed errors are logged, never returned to clients.
func (s *Service) Readiness(ctx context.Context) Readiness {
	rep := Readiness{Status: StatusReady, Checks: make(map[string]CheckResult, len(s.checkers)), CheckedAt: s.now().UTC()}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, c := range s.checkers {
		wg.Go(func() {
			cctx, cancel := context.WithTimeout(ctx, s.checkTimeout)
			defer cancel()
			start := time.Now()
			err := c.Check(cctx)
			res := CheckResult{Status: StatusOK, LatencyMS: time.Since(start).Milliseconds()}
			if err != nil {
				res.Status = StatusUnavailable
				s.log.WarnContext(ctx, "readiness check failed", "check", c.Name(), "error", err)
			}
			mu.Lock()
			rep.Checks[c.Name()] = res
			mu.Unlock()
		})
	}
	wg.Wait()

	for _, r := range rep.Checks {
		if r.Status != StatusOK {
			rep.Status = StatusNotReady
		}
	}
	if s.draining.Load() {
		rep.Status = StatusNotReady
	}
	return rep
}

// PublicVersion returns only the application version (git describe of the build).
func (s *Service) PublicVersion() PublicVersion { return PublicVersion{Version: buildinfo.Version} }

// Version returns build metadata and the current server time in UTC and display timezone.
func (s *Service) Version() Version {
	now, tz := s.now(), s.tz
	if s.location != nil {
		tz = s.location()
	}
	return Version{
		Info:              buildinfo.Get(),
		ServerTimeUTC:     now.UTC(),
		DisplayTimezone:   tz.String(),
		ServerTimeDisplay: now.In(tz).Format(time.RFC3339),
	}
}

// FuncChecker adapts a function to the Checker interface.
type FuncChecker struct {
	CheckName string
	Fn        func(ctx context.Context) error
}

// Name implements Checker.
func (f FuncChecker) Name() string { return f.CheckName }

// Check implements Checker.
func (f FuncChecker) Check(ctx context.Context) error { return f.Fn(ctx) }

// SchemaReader reads the applied migration version.
type SchemaReader interface {
	SchemaVersion(ctx context.Context) (version int64, dirty bool, err error)
}

// ErrSchemaMismatch indicates the database schema is not at the version expected by this binary.
var ErrSchemaMismatch = errors.New("schema version mismatch")

// MigrationChecker verifies the database schema matches the migrations embedded in this binary.
func MigrationChecker(r SchemaReader, expected uint) Checker {
	return FuncChecker{CheckName: "migrations", Fn: func(ctx context.Context) error {
		v, dirty, err := r.SchemaVersion(ctx)
		if err != nil {
			return err
		}
		if dirty || v < 0 || uint64(v) != uint64(expected) {
			return fmt.Errorf("%w: have %d (dirty=%t), want %d", ErrSchemaMismatch, v, dirty, expected)
		}
		return nil
	}}
}
