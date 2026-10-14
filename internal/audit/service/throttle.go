package service

import (
	"context"
	"sync"
	"time"
)

// DefaultThrottleWindow is how long repeated identical denials are folded into one summary event.
const DefaultThrottleWindow = 10 * time.Minute

// defaultThrottleKeys bounds memory: beyond it, new keys share one overflow bucket (still counted, never dropped).
const defaultThrottleKeys = 10000

const overflowKey = "\x00overflow"

// throttle aggregates repeated events (403 authz.denied, CSRF refusals) so a client hammering a forbidden route
// cannot flood the audit log: the first event per key in a window is recorded as is, further ones are only counted
// and written as ONE "<action>.repeated" event (count, first/last time) when the window has passed.
type throttle struct {
	rec    *Recorder
	window time.Duration
	max    int
	now    func() time.Time
	mu     sync.Mutex
	m      map[string]*bucket
}

type bucket struct {
	ev          Event
	start, last time.Time
	suppressed  int
	overflow    bool // shared by many keys: the summary is a system event without actor details
}

func newThrottle(rec *Recorder, window time.Duration, maxKeys int) *throttle {
	return &throttle{rec: rec, window: window, max: maxKeys, now: time.Now, m: map[string]*bucket{}}
}

// RecordAggregated records ev unless an event with the same key was recorded within the window; then it is only
// counted. key must identify actor/client + action + target (the caller builds it).
func (r *Recorder) RecordAggregated(ctx context.Context, key string, ev Event) {
	r.throttle.record(ctx, key, resolve(ctx, ev))
}

// RunThrottle writes the summaries of closed windows periodically and all pending ones when ctx ends.
func (r *Recorder) RunThrottle(ctx context.Context) {
	t := time.NewTicker(r.throttle.window / 2)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			r.throttle.flush(context.WithoutCancel(ctx), true)
			return
		case <-t.C:
			r.throttle.flush(ctx, false)
		}
	}
}

func (t *throttle) record(ctx context.Context, key string, ev Event) {
	now := t.now()
	t.mu.Lock()
	if _, ok := t.m[key]; !ok && len(t.m) >= t.max {
		key = overflowKey
	}
	b := t.m[key]
	if b != nil && now.Sub(b.start) < t.window {
		b.suppressed++
		b.last = now
		t.mu.Unlock()
		return
	}
	var summary *Event
	if b != nil && b.suppressed > 0 {
		s := b.summary()
		summary = &s
	}
	t.m[key] = &bucket{ev: ev, start: now, last: now, overflow: key == overflowKey}
	t.mu.Unlock()
	if summary != nil {
		_ = t.rec.insert(ctx, *summary)
	}
	_ = t.rec.insert(ctx, ev)
}

// flush writes summaries for closed windows (all = every bucket with suppressed events) and forgets closed buckets.
func (t *throttle) flush(ctx context.Context, all bool) {
	now := t.now()
	var out []Event
	t.mu.Lock()
	for k, b := range t.m {
		if !all && now.Sub(b.start) < t.window {
			continue
		}
		if b.suppressed > 0 {
			out = append(out, b.summary())
		}
		delete(t.m, k)
	}
	t.mu.Unlock()
	for _, ev := range out {
		_ = t.rec.insert(ctx, ev)
	}
}

func (b *bucket) summary() Event {
	s := b.ev
	s.Action = b.ev.Action + ".repeated"
	md := make(map[string]any, len(b.ev.Metadata)+4)
	for k, v := range b.ev.Metadata {
		md[k] = v
	}
	if b.overflow {
		s = Event{Action: "audit.throttle_overflow", Result: ResultDenied, ActorType: ActorSystem}
		md = map[string]any{}
	}
	md["repeated"] = b.suppressed
	md["first_at"] = b.start.UTC().Format(time.RFC3339)
	md["last_at"] = b.last.UTC().Format(time.RFC3339)
	s.Metadata = md
	return s
}
