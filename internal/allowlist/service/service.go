// Package service implements Allowed IP for the web panel (v0.3.6, D-078): an allowlist of networks, a mode
// (off / monitor / enforce), the decision for a client IP, the lockout guard (no enforce, and no removal, that would
// shut out the admin making the change), and the automatic rollback from enforce to monitor unless confirmed within
// the confirmation window. The decision uses an in-memory snapshot reloaded every few seconds, so changes made by the
// emergency CLI (another process) take effect without a restart.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/agungharyanto/sentra-dcim/internal/allowlist/repo"
	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
	settings "github.com/agungharyanto/sentra-dcim/internal/settings/service"
)

// Modes.
const (
	ModeOff     = "off"
	ModeMonitor = "monitor"
	ModeEnforce = "enforce"
)

// Audit actions.
const (
	ActionAdd      = "allowlist.add"
	ActionUpdate   = "allowlist.update"
	ActionRemove   = "allowlist.remove"
	ActionMode     = "allowlist.mode"
	ActionConfirm  = "allowlist.confirm"
	ActionRollback = "allowlist.rollback"
	ActionOutside  = "security.allowlist_outside" // monitor: request from outside the list (allowed)
	ActionBlocked  = "security.allowlist_blocked" // enforce: request refused
)

// Limits and timing.
const (
	ConfirmWindow  = 2 * time.Minute
	ReloadInterval = 5 * time.Second
	MaxEntries     = 500
	MaxLabel       = 100
)

// Errors.
var (
	ErrDuplicate   = errors.New("network already in the list")
	ErrNotFound    = errors.New("entry not found")
	ErrLockout     = errors.New("change would lock out the requesting address")
	ErrNoPending   = errors.New("no enforce change waiting for confirmation")
	ErrTooMany     = errors.New("too many entries")
	ErrInvalidMode = errors.New("invalid mode")
)

// ValidationError names an invalid field.
type ValidationError struct{ Field, Reason string }

func (e *ValidationError) Error() string { return e.Field + ": " + e.Reason }

// ParseNetwork accepts an IPv4/IPv6 address or CIDR and returns the canonical network: single addresses become /32
// or /128, IPv4-mapped IPv6 is unmapped, host bits must be zero (10.0.0.0/24, not 10.0.0.5/24) so the stored value
// is exactly what the admin meant. Networks broader than /8 (IPv4) or /16 (IPv6) are refused.
func ParseNetwork(s string) (netip.Prefix, error) {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 64 {
		return netip.Prefix{}, errors.New("enter an IP address or network (CIDR)")
	}
	if strings.Contains(s, "%") {
		return netip.Prefix{}, errors.New("zones (%) are not allowed")
	}
	var p netip.Prefix
	if strings.Contains(s, "/") {
		var err error
		if p, err = netip.ParsePrefix(s); err != nil {
			return netip.Prefix{}, errors.New("not a valid IP network (CIDR)")
		}
		if p.Addr().Is4In6() {
			if p.Bits() < 96 {
				return netip.Prefix{}, errors.New("not a valid IP network (CIDR)")
			}
			p = netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96)
		}
		if p.Masked() != p {
			return netip.Prefix{}, fmt.Errorf("host bits set: did you mean %s?", p.Masked())
		}
	} else {
		a, err := netip.ParseAddr(s)
		if err != nil {
			return netip.Prefix{}, errors.New("not a valid IP address")
		}
		a = a.Unmap()
		p = netip.PrefixFrom(a, a.BitLen())
	}
	if (p.Addr().Is4() && p.Bits() < 8) || (p.Addr().Is6() && p.Bits() < 16) {
		return netip.Prefix{}, errors.New("network too broad (minimum /8 for IPv4, /16 for IPv6)")
	}
	return p, nil
}

// Snapshot is the state used for decisions.
type Snapshot struct {
	Mode     string
	Networks []netip.Prefix
	Pending  time.Time // zero = no confirmation pending
	Loaded   time.Time
}

// Covers reports whether ip is inside one of the networks.
func (s *Snapshot) Covers(ip netip.Addr) bool {
	ip = ip.Unmap()
	for _, n := range s.Networks {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// DB is what the service needs (pgxpool.Pool).
type DB interface {
	repo.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Service owns the allowlist.
type Service struct {
	db   DB
	reg  *settings.Registry
	rec  *audit.Recorder
	log  *slog.Logger
	now  func() time.Time
	snap atomic.Pointer[Snapshot]
}

// New creates a Service. Call Reload (or Run) before using decisions; until then the mode is off.
func New(db DB, set *settings.Service, rec *audit.Recorder, log *slog.Logger) *Service {
	s := &Service{db: db, reg: set.Registry(), rec: rec, log: log, now: time.Now}
	s.snap.Store(&Snapshot{Mode: ModeOff})
	return s
}

// SetClock replaces the clock (tests).
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Current returns the last loaded snapshot.
func (s *Service) Current() *Snapshot { return s.snap.Load() }

// Reload reads mode, deadline and networks from the database.
func (s *Service) Reload(ctx context.Context) error {
	snap, err := s.load(ctx, s.db)
	if err != nil {
		return err
	}
	snap.Loaded = s.now()
	s.snap.Store(snap)
	return nil
}

func (s *Service) load(ctx context.Context, db repo.DBTX) (*Snapshot, error) {
	q := repo.New(db)
	rows, err := q.ListAllowed(ctx)
	if err != nil {
		return nil, fmt.Errorf("list allowed: %w", err)
	}
	snap := &Snapshot{Mode: ModeOff}
	for _, r := range rows {
		if p, err := netip.ParsePrefix(r.Cidr); err == nil {
			snap.Networks = append(snap.Networks, p)
		}
	}
	mode, err := settingValue(ctx, db, s.reg, settings.KeyAllowlistMode)
	if err != nil {
		return nil, err
	}
	snap.Mode = mode
	pending, err := settingValue(ctx, db, s.reg, settings.KeyAllowlistPending)
	if err != nil {
		return nil, err
	}
	if pending != "" {
		if t, err := time.Parse(time.RFC3339, pending); err == nil {
			snap.Pending = t
		}
	}
	return snap, nil
}

// settingValue reads a managed string setting through db (pool or transaction).
func settingValue(ctx context.Context, db repo.DBTX, reg *settings.Registry, key string) (string, error) {
	d, _ := reg.Def(key)
	var raw []byte
	err := db.QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return d.Default.(string), nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", key, err)
	}
	v := strings.Trim(string(raw), `"`)
	if key == settings.KeyAllowlistMode && v != ModeOff && v != ModeMonitor && v != ModeEnforce {
		return ModeOff, nil
	}
	return v, nil
}

// Run reloads periodically and performs due rollbacks until ctx ends.
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(ReloadInterval)
	defer t.Stop()
	for {
		if err := s.Tick(ctx); err != nil && ctx.Err() == nil {
			s.log.ErrorContext(ctx, "allowlist reload", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Tick rolls back an unconfirmed enforce whose window has passed, then reloads.
func (s *Service) Tick(ctx context.Context) error {
	if err := s.rollbackIfDue(ctx); err != nil {
		return err
	}
	return s.Reload(ctx)
}

// Decision is the outcome for one request.
type Decision struct {
	Allow   bool
	Outside bool // the client IP is not in the list (audited in monitor and enforce)
	Mode    string
}

// Decide applies the mode to a client IP. An invalid/unknown IP counts as outside.
func (s *Service) Decide(ip netip.Addr) Decision {
	snap := s.Current()
	switch snap.Mode {
	case ModeMonitor:
		return Decision{Allow: true, Outside: !ip.IsValid() || !snap.Covers(ip), Mode: snap.Mode}
	case ModeEnforce:
		in := ip.IsValid() && snap.Covers(ip)
		return Decision{Allow: in, Outside: !in, Mode: snap.Mode}
	}
	return Decision{Allow: true, Mode: ModeOff}
}

// RecordOutside audits a request from outside the list (aggregated per IP and path so it cannot flood the log).
func (s *Service) RecordOutside(ctx context.Context, d Decision, ip netip.Addr, method, path string, meta audit.Meta) {
	action, result := ActionOutside, audit.ResultSuccess
	if !d.Allow {
		action, result = ActionBlocked, audit.ResultDenied
	}
	if len(path) > 128 {
		path = path[:128]
	}
	if ip.IsValid() {
		a := ip
		meta.IP = &a
	}
	s.rec.RecordAggregated(ctx, "allowlist|"+ip.String()+"|"+path, audit.Event{Action: action, Result: result,
		ActorType: audit.ActorAnonymous, TargetType: "route", TargetID: method + " " + path, Meta: meta,
		Metadata: map[string]any{"mode": d.Mode}})
}

// Entry is one allowed network.
type Entry struct {
	ID        uuid.UUID  `json:"id"`
	CIDR      string     `json:"cidr"`
	Label     string     `json:"label"`
	CreatedAt time.Time  `json:"created_at"`
	CreatedBy *uuid.UUID `json:"created_by"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// State is the full view for the API.
type State struct {
	Mode         string     `json:"mode"`
	PendingUntil *time.Time `json:"pending_until"` // enforce waiting for confirmation (auto-rollback deadline)
	Entries      []Entry    `json:"entries"`
}

// State reads the stored state (not the cached snapshot).
func (s *Service) State(ctx context.Context) (State, error) {
	snap, err := s.load(ctx, s.db)
	if err != nil {
		return State{}, err
	}
	rows, err := repo.New(s.db).ListAllowed(ctx)
	if err != nil {
		return State{}, err
	}
	st := State{Mode: snap.Mode, Entries: make([]Entry, 0, len(rows))}
	if !snap.Pending.IsZero() {
		t := snap.Pending.UTC()
		st.PendingUntil = &t
	}
	for _, r := range rows {
		st.Entries = append(st.Entries, Entry{ID: r.ID, CIDR: r.Cidr, Label: r.Label, CreatedAt: r.CreatedAt.Time.UTC(),
			CreatedBy: r.CreatedBy, UpdatedAt: r.UpdatedAt.Time.UTC()})
	}
	return st, nil
}

func actorID(ctx context.Context) *uuid.UUID {
	if p, ok := principal.From(ctx); ok {
		id := p.ID
		return &id
	}
	return nil
}

// event fills the actor for server-side commands (no session → system "cli").
func event(ctx context.Context, ev audit.Event) audit.Event {
	if _, ok := principal.From(ctx); !ok && ev.ActorType == "" {
		ev.ActorType, ev.ActorUsername = audit.ActorSystem, "cli"
	}
	return ev
}

func (s *Service) inTx(ctx context.Context, fn func(tx pgx.Tx, q *repo.Queries) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := repo.New(tx)
	if err := q.LockAllowlist(ctx); err != nil {
		return err
	}
	if err := fn(tx, q); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Add stores a network. label is free text (max 100 characters).
func (s *Service) Add(ctx context.Context, cidr, label string, meta audit.Meta) (Entry, error) {
	p, err := ParseNetwork(cidr)
	if err != nil {
		return Entry{}, &ValidationError{Field: "cidr", Reason: err.Error()}
	}
	label = strings.TrimSpace(label)
	if utf8.RuneCountInString(label) > MaxLabel || strings.ContainsAny(label, "\r\n\t") {
		return Entry{}, &ValidationError{Field: "label", Reason: "at most 100 characters, one line"}
	}
	var e Entry
	err = s.inTx(ctx, func(_ pgx.Tx, q *repo.Queries) error {
		n, err := q.CountAllowed(ctx)
		if err != nil {
			return err
		}
		if n >= MaxEntries {
			return ErrTooMany
		}
		r, err := q.InsertAllowed(ctx, repo.InsertAllowedParams{Cidr: p, Label: label, CreatedBy: actorID(ctx)})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrDuplicate
		}
		if err != nil {
			return err
		}
		e = Entry{ID: r.ID, CIDR: r.Cidr, Label: r.Label, CreatedAt: r.CreatedAt.Time.UTC(), CreatedBy: r.CreatedBy, UpdatedAt: r.UpdatedAt.Time.UTC()}
		return nil
	})
	if err != nil {
		return Entry{}, err
	}
	_ = s.rec.Record(ctx, event(ctx, audit.Event{Action: ActionAdd, Result: audit.ResultSuccess, TargetType: "allowed_ip",
		TargetID: e.CIDR, Meta: meta, Metadata: map[string]any{"cidr": e.CIDR, "label": e.Label}}))
	return e, s.Reload(ctx)
}

// UpdateLabel changes the label of an entry.
func (s *Service) UpdateLabel(ctx context.Context, id uuid.UUID, label string, meta audit.Meta) (Entry, error) {
	label = strings.TrimSpace(label)
	if utf8.RuneCountInString(label) > MaxLabel || strings.ContainsAny(label, "\r\n\t") {
		return Entry{}, &ValidationError{Field: "label", Reason: "at most 100 characters, one line"}
	}
	var e Entry
	var old string
	err := s.inTx(ctx, func(_ pgx.Tx, q *repo.Queries) error {
		cur, err := q.GetAllowed(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		old = cur.Label
		r, err := q.UpdateAllowedLabel(ctx, repo.UpdateAllowedLabelParams{ID: id, Label: label})
		if err != nil {
			return err
		}
		e = Entry{ID: r.ID, CIDR: r.Cidr, Label: r.Label, CreatedAt: r.CreatedAt.Time.UTC(), CreatedBy: r.CreatedBy, UpdatedAt: r.UpdatedAt.Time.UTC()}
		return nil
	})
	if err != nil {
		return Entry{}, err
	}
	if old != label {
		_ = s.rec.Record(ctx, event(ctx, audit.Event{Action: ActionUpdate, Result: audit.ResultSuccess, TargetType: "allowed_ip",
			TargetID: e.CIDR, Meta: meta, Metadata: map[string]any{"cidr": e.CIDR, "old": old, "new": label}}))
	}
	return e, nil
}

// Remove deletes an entry by id or, from the CLI, by network. requester is the admin's own address (invalid for the
// CLI): while enforcing, removing the last network that covers it is refused.
func (s *Service) Remove(ctx context.Context, id *uuid.UUID, cidr string, requester netip.Addr, meta audit.Meta) (string, error) {
	var removed, label string
	var refused bool
	err := s.inTx(ctx, func(tx pgx.Tx, q *repo.Queries) error {
		if id != nil {
			r, err := q.DeleteAllowed(ctx, *id)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			if err != nil {
				return err
			}
			removed, label = r.Cidr, r.Label
		} else {
			p, err := ParseNetwork(cidr)
			if err != nil {
				return &ValidationError{Field: "cidr", Reason: err.Error()}
			}
			r, err := q.DeleteAllowedByCIDR(ctx, p)
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			if err != nil {
				return err
			}
			removed, label = p.String(), r.Label
		}
		after, err := s.load(ctx, tx)
		if err != nil {
			return err
		}
		if after.Mode == ModeEnforce && requester.IsValid() && !after.Covers(requester) {
			refused = true
			return ErrLockout
		}
		return nil
	})
	if refused {
		_ = s.rec.Record(ctx, event(ctx, audit.Event{Action: ActionRemove, Result: audit.ResultDenied, TargetType: "allowed_ip",
			TargetID: removed, Meta: meta, Metadata: map[string]any{"reason": "lockout"}}))
	}
	if err != nil {
		return "", err
	}
	_ = s.rec.Record(ctx, event(ctx, audit.Event{Action: ActionRemove, Result: audit.ResultSuccess, TargetType: "allowed_ip",
		TargetID: removed, Meta: meta, Metadata: map[string]any{"cidr": removed, "label": label}}))
	return removed, s.Reload(ctx)
}

// ModeOptions controls a mode change.
type ModeOptions struct {
	// Requester is the admin's own address. Switching to enforce requires it to be covered (lockout guard); the
	// CLI passes an invalid address and Force.
	Requester netip.Addr
	// Force (CLI only) switches to enforce without the guard and without the auto-rollback window.
	Force bool
}

// SetMode changes the mode. Switching to enforce through the API starts the confirmation window: unless Confirm is
// called before it ends, the mode returns to monitor.
func (s *Service) SetMode(ctx context.Context, mode string, opt ModeOptions, meta audit.Meta) (State, error) {
	if mode != ModeOff && mode != ModeMonitor && mode != ModeEnforce {
		return State{}, &ValidationError{Field: "mode", Reason: "one of off, monitor, enforce"}
	}
	var old string
	var pending time.Time
	var refused bool
	err := s.inTx(ctx, func(tx pgx.Tx, _ *repo.Queries) error {
		cur, err := s.load(ctx, tx)
		if err != nil {
			return err
		}
		old = cur.Mode
		if mode == ModeEnforce && !opt.Force {
			if !opt.Requester.IsValid() || !cur.Covers(opt.Requester) {
				refused = true
				return ErrLockout
			}
		}
		var pendingValue any // nil = clear
		switch {
		case mode == ModeEnforce && old != ModeEnforce && !opt.Force:
			pending = s.now().Add(ConfirmWindow).UTC().Truncate(time.Second)
			pendingValue = pending.Format(time.RFC3339)
		case mode == ModeEnforce && old == ModeEnforce:
			pending = cur.Pending
			if !pending.IsZero() {
				pendingValue = pending.UTC().Format(time.RFC3339)
			}
		}
		if err := settings.SetManaged(ctx, tx, s.reg, settings.KeyAllowlistMode, modeValue(mode), actorID(ctx)); err != nil {
			return err
		}
		return settings.SetManaged(ctx, tx, s.reg, settings.KeyAllowlistPending, pendingValue, actorID(ctx))
	})
	if refused {
		_ = s.rec.Record(ctx, event(ctx, audit.Event{Action: ActionMode, Result: audit.ResultDenied, TargetType: "allowlist",
			TargetID: "mode", Meta: meta, Metadata: map[string]any{"old": old, "new": mode, "reason": "lockout",
				"requester_covered": false}}))
	}
	if err != nil {
		return State{}, err
	}
	if old != mode {
		md := map[string]any{"old": old, "new": mode, "force": opt.Force}
		if !pending.IsZero() {
			md["confirm_by"] = pending.Format(time.RFC3339)
		}
		_ = s.rec.Record(ctx, event(ctx, audit.Event{Action: ActionMode, Result: audit.ResultSuccess, TargetType: "allowlist",
			TargetID: "mode", Meta: meta, Metadata: md}))
	}
	if err := s.Reload(ctx); err != nil {
		return State{}, err
	}
	return s.State(ctx)
}

// modeValue stores off as the default (no row).
func modeValue(mode string) any {
	if mode == ModeOff {
		return nil
	}
	return mode
}

// Confirm keeps enforce. It is called by an admin whose request has just passed the enforcement, which proves the
// new list still lets them in.
func (s *Service) Confirm(ctx context.Context, requester netip.Addr, meta audit.Meta) (State, error) {
	err := s.inTx(ctx, func(tx pgx.Tx, _ *repo.Queries) error {
		cur, err := s.load(ctx, tx)
		if err != nil {
			return err
		}
		if cur.Mode != ModeEnforce || cur.Pending.IsZero() {
			return ErrNoPending
		}
		if !requester.IsValid() || !cur.Covers(requester) {
			return ErrLockout
		}
		return settings.SetManaged(ctx, tx, s.reg, settings.KeyAllowlistPending, nil, actorID(ctx))
	})
	if err != nil {
		return State{}, err
	}
	_ = s.rec.Record(ctx, event(ctx, audit.Event{Action: ActionConfirm, Result: audit.ResultSuccess, TargetType: "allowlist",
		TargetID: "mode", Meta: meta, Metadata: map[string]any{"mode": ModeEnforce}}))
	if err := s.Reload(ctx); err != nil {
		return State{}, err
	}
	return s.State(ctx)
}

// rollbackIfDue switches an unconfirmed enforce back to monitor once its window has passed.
func (s *Service) rollbackIfDue(ctx context.Context) error {
	var rolled bool
	var deadline time.Time
	err := s.inTx(ctx, func(tx pgx.Tx, _ *repo.Queries) error {
		cur, err := s.load(ctx, tx)
		if err != nil {
			return err
		}
		if cur.Mode != ModeEnforce || cur.Pending.IsZero() || s.now().Before(cur.Pending) {
			return nil
		}
		deadline = cur.Pending
		if err := settings.SetManaged(ctx, tx, s.reg, settings.KeyAllowlistMode, ModeMonitor, nil); err != nil {
			return err
		}
		if err := settings.SetManaged(ctx, tx, s.reg, settings.KeyAllowlistPending, nil, nil); err != nil {
			return err
		}
		rolled = true
		return nil
	})
	if err != nil {
		return err
	}
	if rolled {
		s.log.WarnContext(ctx, "allowlist enforce not confirmed in time: rolled back to monitor", "deadline", deadline)
		_ = s.rec.Record(ctx, audit.Event{Action: ActionRollback, Result: audit.ResultSuccess, ActorType: audit.ActorSystem,
			ActorUsername: "sentra", TargetType: "allowlist", TargetID: "mode",
			Metadata: map[string]any{"old": ModeEnforce, "new": ModeMonitor, "deadline": deadline.UTC().Format(time.RFC3339)}})
	}
	return nil
}
