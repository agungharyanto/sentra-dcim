package chain

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// Sink is told about a broken chain. The default logs at error level; the Alarm Engine (v0.5) will implement it to
// raise an alarm.
type Sink interface {
	ChainBroken(ctx context.Context, r Result)
}

// LogSink logs a broken chain at error level (never row contents).
type LogSink struct{ Log *slog.Logger }

// ChainBroken implements Sink.
func (s LogSink) ChainBroken(ctx context.Context, r Result) {
	s.Log.ErrorContext(ctx, "audit chain verification FAILED", "broken_seq", r.BrokenSeq, "reason", r.Reason,
		"checked", r.Checked, "checkpoint_seq", r.CheckpointSeq)
}

// ErrBusy means a verification is already running.
var ErrBusy = errors.New("verification already running")

// Monitor verifies the chain daily (and on demand) and keeps the last result for the API.
type Monitor struct {
	db    DB
	log   *slog.Logger
	sink  Sink
	every time.Duration
	run   sync.Mutex // one verification at a time
	busy  atomic.Bool
	mu    sync.RWMutex
	last  *Result
}

// NewMonitor creates a Monitor; sink nil = LogSink.
func NewMonitor(db DB, log *slog.Logger, sink Sink, every time.Duration) *Monitor {
	if sink == nil {
		sink = LogSink{Log: log}
	}
	return &Monitor{db: db, log: log, sink: sink, every: every}
}

// Run verifies at start and then every interval until ctx ends.
func (m *Monitor) Run(ctx context.Context) {
	t := time.NewTicker(m.every)
	defer t.Stop()
	for {
		if _, err := m.VerifyNow(ctx); err != nil && ctx.Err() == nil && !errors.Is(err, ErrBusy) {
			m.log.ErrorContext(ctx, "audit chain verification could not run", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// VerifyNow runs a verification (without anchors: those live in the backups, see `sentra audit verify`).
func (m *Monitor) VerifyNow(ctx context.Context) (Result, error) {
	if !m.run.TryLock() {
		return Result{}, ErrBusy
	}
	defer m.run.Unlock()
	m.busy.Store(true)
	defer m.busy.Store(false)
	res, err := Verify(ctx, m.db, nil, time.Now())
	if err != nil {
		return res, err
	}
	m.mu.Lock()
	m.last = &res
	m.mu.Unlock()
	if !res.OK {
		m.sink.ChainBroken(ctx, res)
	} else {
		m.log.InfoContext(ctx, "audit chain verified", "checked", res.Checked, "head_seq", res.HeadSeq)
	}
	return res, nil
}

// Running reports whether a verification is in progress.
func (m *Monitor) Running() bool { return m.busy.Load() }

// Last returns the latest result (nil before the first run).
func (m *Monitor) Last() *Result {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.last == nil {
		return nil
	}
	r := *m.last
	return &r
}
