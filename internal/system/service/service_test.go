package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
)

func ok(name string) Checker {
	return FuncChecker{CheckName: name, Fn: func(context.Context) error { return nil }}
}

func failing(name string) Checker {
	return FuncChecker{CheckName: name, Fn: func(context.Context) error { return errors.New("dial tcp 10.0.0.5:5432: refused") }}
}

func wita(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Makassar")
	if err != nil {
		t.Fatal(err)
	}
	return loc
}

func TestReadinessAllOK(t *testing.T) {
	s := New(logging.Discard(), wita(t), ok("database"), ok("redis"))
	rep := s.Readiness(context.Background())
	if rep.Status != StatusReady || len(rep.Checks) != 2 {
		t.Fatalf("unexpected report: %+v", rep)
	}
}

func TestReadinessFailureHidesDetails(t *testing.T) {
	s := New(logging.Discard(), wita(t), ok("database"), failing("redis"))
	rep := s.Readiness(context.Background())
	if rep.Status != StatusNotReady {
		t.Fatalf("status = %s", rep.Status)
	}
	if rep.Checks["redis"].Status != StatusUnavailable || rep.Checks["database"].Status != StatusOK {
		t.Fatalf("checks = %+v", rep.Checks)
	}
}

func TestReadinessTimeout(t *testing.T) {
	slow := FuncChecker{CheckName: "slow", Fn: func(ctx context.Context) error { <-ctx.Done(); return ctx.Err() }}
	s := New(logging.Discard(), wita(t), slow)
	s.checkTimeout = 50 * time.Millisecond
	start := time.Now()
	rep := s.Readiness(context.Background())
	if rep.Status != StatusNotReady || time.Since(start) > time.Second {
		t.Fatalf("timeout not enforced: %+v", rep)
	}
}

func TestReadinessDraining(t *testing.T) {
	s := New(logging.Discard(), wita(t), ok("database"))
	s.SetDraining()
	if rep := s.Readiness(context.Background()); rep.Status != StatusNotReady {
		t.Fatalf("draining should be not_ready, got %s", rep.Status)
	}
}

func TestVersionTimezones(t *testing.T) {
	s := New(logging.Discard(), wita(t))
	fixed := time.Date(2026, 10, 12, 4, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return fixed }
	v := s.Version()
	if !v.ServerTimeUTC.Equal(fixed) || v.ServerTimeUTC.Location() != time.UTC {
		t.Fatalf("utc time = %v", v.ServerTimeUTC)
	}
	if v.ServerTimeDisplay != "2026-10-12T12:00:00+08:00" || v.DisplayTimezone != "Asia/Makassar" {
		t.Fatalf("display = %s %s", v.ServerTimeDisplay, v.DisplayTimezone)
	}
}

type fakeSchema struct {
	v     int64
	dirty bool
	err   error
}

func (f fakeSchema) SchemaVersion(context.Context) (int64, bool, error) { return f.v, f.dirty, f.err }

func TestMigrationChecker(t *testing.T) {
	cases := []struct {
		name    string
		schema  fakeSchema
		wantErr bool
	}{
		{"match", fakeSchema{v: 1}, false},
		{"behind", fakeSchema{v: 0}, true},
		{"dirty", fakeSchema{v: 1, dirty: true}, true},
		{"error", fakeSchema{err: errors.New("no table")}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := MigrationChecker(tc.schema, 1).Check(context.Background())
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %t", err, tc.wantErr)
			}
		})
	}
}
