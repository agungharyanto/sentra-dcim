// Package migrator applies embedded SQL migrations using golang-migrate.
package migrator

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"regexp"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // pgx5:// driver
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/agungharyanto/sentra-dcim/migrations"
)

// Migrator wraps golang-migrate with the embedded migration source.
type Migrator struct {
	m   *migrate.Migrate
	log *slog.Logger
}

// New opens a migrator for the given postgres:// DSN.
func New(dsn string, log *slog.Logger) (*Migrator, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("open migration source: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, toPgx5(dsn))
	if err != nil {
		return nil, fmt.Errorf("open migrator: %w", err)
	}
	m.Log = migrateLogger{log}
	return &Migrator{m: m, log: log}, nil
}

// Close releases the migrator resources.
func (mg *Migrator) Close() error {
	srcErr, dbErr := mg.m.Close()
	return errors.Join(srcErr, dbErr)
}

// Up applies all pending migrations.
func (mg *Migrator) Up() error { return ignoreNoChange(mg.m.Up()) }

// Down rolls back n migrations.
func (mg *Migrator) Down(n int) error {
	if n <= 0 {
		return fmt.Errorf("steps must be positive, got %d", n)
	}
	return ignoreNoChange(mg.m.Steps(-n))
}

// Goto migrates up or down to the given version. Version 0 rolls back everything.
func (mg *Migrator) Goto(version uint) error {
	if version == 0 {
		return ignoreNoChange(mg.m.Down())
	}
	return ignoreNoChange(mg.m.Migrate(version))
}

// Force sets the version without running migrations (to recover from a dirty state).
func (mg *Migrator) Force(version int) error { return mg.m.Force(version) }

// Version returns the current schema version. ok is false when no migration has been applied.
func (mg *Migrator) Version() (version uint, dirty, ok bool, err error) {
	v, d, err := mg.m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, false, nil
	}
	if err != nil {
		return 0, false, false, err
	}
	return v, d, true, nil
}

var migrationName = regexp.MustCompile(`^(\d+)_.*\.up\.sql$`)

// Latest returns the highest migration version embedded in the binary.
func Latest() (uint, error) {
	entries, err := fs.ReadDir(migrations.FS, ".")
	if err != nil {
		return 0, err
	}
	var latest uint64
	for _, e := range entries {
		if m := migrationName.FindStringSubmatch(e.Name()); m != nil {
			v, err := strconv.ParseUint(m[1], 10, 64)
			if err != nil {
				return 0, err
			}
			latest = max(latest, v)
		}
	}
	return uint(latest), nil
}

func ignoreNoChange(err error) error {
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	return err
}

func toPgx5(dsn string) string {
	const prefix = "postgres://"
	if len(dsn) > len(prefix) && dsn[:len(prefix)] == prefix {
		return "pgx5://" + dsn[len(prefix):]
	}
	return dsn
}

type migrateLogger struct{ log *slog.Logger }

func (l migrateLogger) Printf(format string, v ...any) {
	l.log.Info(fmt.Sprintf(format, v...), "component", "migrate")
}
func (l migrateLogger) Verbose() bool { return false }
