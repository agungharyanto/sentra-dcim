package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
	"github.com/agungharyanto/sentra-dcim/internal/settings/repo"
)

// Audit actions.
const (
	ActionUpdate            = "settings.update"
	ActionPreferencesUpdate = "user.preferences.update"
)

// Errors.
var (
	ErrUnknownGroup = errors.New("unknown settings group")
)

// ValidationError names the invalid key (without group prefix) and why.
type ValidationError struct{ Field, Reason string }

func (e *ValidationError) Error() string { return e.Field + ": " + e.Reason }

// DB is what the service needs (pgxpool.Pool).
type DB interface {
	repo.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

// Service reads and writes settings and preferences.
type Service struct {
	db  DB
	reg *Registry
	rec *audit.Recorder
	log *slog.Logger
	pc  policyCache
}

// New creates a Service.
func New(db DB, reg *Registry, rec *audit.Recorder, log *slog.Logger) *Service {
	return &Service{db: db, reg: reg, rec: rec, log: log}
}

// Registry returns the key registry.
func (s *Service) Registry() *Registry { return s.reg }

// Item is one key of a group as returned by the API.
type Item struct {
	Def
	Value     any        `json:"value"`      // effective value (default when not stored)
	IsDefault bool       `json:"is_default"` // no stored value
	UpdatedAt *time.Time `json:"updated_at"`
	UpdatedBy *uuid.UUID `json:"updated_by"`
}

// Group returns every key of a group with its current value.
func (s *Service) Group(ctx context.Context, group string) ([]Item, error) {
	defs := s.reg.Group(group)
	if len(defs) == 0 {
		return nil, ErrUnknownGroup
	}
	rows, err := repo.New(s.db).ListSettings(ctx, group+".")
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	stored := map[string]repo.Setting{}
	for _, r := range rows {
		stored[r.Key] = r
	}
	out := make([]Item, 0, len(defs))
	for _, d := range defs {
		it := Item{Def: d, Value: d.Default, IsDefault: true}
		if r, ok := stored[d.Key]; ok {
			if v, err := d.decode(r.Value); err == nil {
				t := r.UpdatedAt.Time.UTC()
				it.Value, it.IsDefault, it.UpdatedAt, it.UpdatedBy = v, false, &t, r.UpdatedBy
			} else {
				// A stored value no longer valid (e.g. a zone removed from tzdata): fall back, never fail the read.
				s.log.WarnContext(ctx, "stored setting invalid, using default", "key", d.Key, "error", err)
			}
		}
		out = append(out, it)
	}
	return out, nil
}

// Update applies changes to a group: name → JSON value, or JSON null to reset to the default. All values are
// validated before anything is written; each real change is audited with its old and new value.
func (s *Service) Update(ctx context.Context, group string, changes map[string]json.RawMessage, meta audit.Meta) ([]Item, error) {
	if len(s.reg.Group(group)) == 0 {
		return nil, ErrUnknownGroup
	}
	parsed, err := s.parse(group, changes, false)
	if err != nil {
		return nil, err
	}
	var actor *uuid.UUID
	if p, ok := principal.From(ctx); ok {
		id := p.ID
		actor = &id
	}
	var events []audit.Event
	err = s.inTx(ctx, func(q *repo.Queries) error {
		if err := q.LockSettings(ctx, "system"); err != nil {
			return err
		}
		before, err := s.groupWith(ctx, q, group)
		if err != nil {
			return err
		}
		after := map[string]any{}
		for _, it := range before {
			after[it.Name()] = it.Value
		}
		for _, c := range parsed {
			if c.reset {
				after[c.def.Name()] = c.def.Default
			} else {
				after[c.def.Name()] = c.value
			}
		}
		if field, reason := s.reg.CheckGroup(group, after); field != "" {
			return &ValidationError{Field: field, Reason: reason}
		}
		for _, c := range parsed {
			old := before[c.def.Key]
			if c.reset {
				if old.IsDefault {
					continue
				}
				if err := q.DeleteSetting(ctx, c.def.Key); err != nil {
					return err
				}
			} else {
				if !old.IsDefault && reflect.DeepEqual(old.Value, c.value) {
					continue
				}
				if err := q.UpsertSetting(ctx, repo.UpsertSettingParams{Key: c.def.Key, Value: mustJSON(c.value), UpdatedBy: actor}); err != nil {
					return err
				}
			}
			newValue := c.value
			if c.reset {
				newValue = c.def.Default
			}
			events = append(events, audit.Event{Action: ActionUpdate, Result: audit.ResultSuccess, TargetType: "setting",
				TargetID: c.def.Key, Meta: meta, Metadata: map[string]any{"old": old.Value, "new": newValue,
					"old_is_default": old.IsDefault, "new_is_default": c.reset}})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.invalidatePolicy()
	for _, ev := range events {
		_ = s.rec.Record(ctx, ev)
	}
	return s.Group(ctx, group)
}

type change struct {
	def   Def
	value any
	reset bool
}

// parse validates a change set; users=true restricts it to user-overridable keys.
func (s *Service) parse(group string, changes map[string]json.RawMessage, users bool) ([]change, error) {
	if len(changes) == 0 {
		return nil, &ValidationError{Field: "values", Reason: "no changes"}
	}
	var out []change
	for name, raw := range changes {
		key := name
		if group != "" {
			key = group + "." + name
		}
		d, ok := s.reg.Def(key)
		if !ok || (users && !d.UserOverridable) {
			return nil, &ValidationError{Field: name, Reason: "unknown setting"}
		}
		if d.Managed {
			return nil, &ValidationError{Field: name, Reason: "managed setting: change it through its own endpoint"}
		}
		if string(raw) == "null" {
			out = append(out, change{def: d, reset: true})
			continue
		}
		v, err := d.decode(raw)
		if err != nil {
			return nil, &ValidationError{Field: name, Reason: err.Error()}
		}
		out = append(out, change{def: d, value: v})
	}
	return out, nil
}

func (s *Service) groupWith(ctx context.Context, q *repo.Queries, group string) (map[string]Item, error) {
	rows, err := q.ListSettings(ctx, group+".")
	if err != nil {
		return nil, err
	}
	out := map[string]Item{}
	for _, d := range s.reg.Group(group) {
		out[d.Key] = Item{Def: d, Value: d.Default, IsDefault: true}
	}
	for _, r := range rows {
		if d, ok := s.reg.Def(r.Key); ok {
			if v, err := d.decode(r.Value); err == nil {
				out[d.Key] = Item{Def: d, Value: v}
			}
		}
	}
	return out, nil
}

func (s *Service) inTx(ctx context.Context, fn func(q *repo.Queries) error) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(repo.New(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Value returns the current value of one key (its default when not stored or no longer valid).
func (s *Service) Value(ctx context.Context, key string) (any, error) {
	d, ok := s.reg.Def(key)
	if !ok {
		return nil, fmt.Errorf("unknown setting %s", key)
	}
	rows, err := repo.New(s.db).ListSettings(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("read setting: %w", err)
	}
	for _, r := range rows {
		if r.Key == key {
			if v, err := d.decode(r.Value); err == nil {
				return v, nil
			}
		}
	}
	return d.Default, nil
}

// SetManaged writes managed keys inside the caller's transaction (no audit: the owning module audits its own
// action). value nil = back to the default.
func SetManaged(ctx context.Context, tx repo.DBTX, reg *Registry, key string, value any, by *uuid.UUID) error {
	d, ok := reg.Def(key)
	if !ok || !d.Managed {
		return fmt.Errorf("%s is not a managed setting", key)
	}
	q := repo.New(tx)
	if value == nil {
		return q.DeleteSetting(ctx, key)
	}
	if _, err := d.decode(mustJSON(value)); err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	return q.UpsertSetting(ctx, repo.UpsertSettingParams{Key: key, Value: mustJSON(value), UpdatedBy: by})
}

// Regional is the effective regional configuration of a user (or the system).
type Regional struct {
	Language   string `json:"language"`
	Timezone   string `json:"timezone"`
	TimeFormat string `json:"time_format"`
	DateFormat string `json:"date_format"`
}

// SystemRegional returns the system regional settings.
func (s *Service) SystemRegional(ctx context.Context) (Regional, error) {
	items, err := s.Group(ctx, "regional")
	if err != nil {
		return Regional{}, err
	}
	return regionalFrom(func(key string) any {
		for _, it := range items {
			if it.Key == key {
				return it.Value
			}
		}
		return nil
	}), nil
}

func regionalFrom(get func(string) any) Regional {
	str := func(k string) string { v, _ := get(k).(string); return v }
	return Regional{Language: str(KeyLanguage), Timezone: str(KeyTimezone), TimeFormat: str(KeyTimeFormat), DateFormat: str(KeyDateFormat)}
}

// SystemLocation returns the system display time zone (the .env default when unreadable).
func (s *Service) SystemLocation(ctx context.Context) *time.Location {
	r, err := s.SystemRegional(ctx)
	if err == nil {
		if loc, err := time.LoadLocation(r.Timezone); err == nil {
			return loc
		}
	}
	d, _ := s.reg.Def(KeyTimezone)
	loc, err := time.LoadLocation(d.Default.(string))
	if err != nil {
		return time.UTC
	}
	return loc
}

// LocationFor returns the effective display time zone of the request principal (system zone without one).
func (s *Service) LocationFor(ctx context.Context) *time.Location {
	if p, ok := principal.From(ctx); ok {
		if eff, _, err := s.EffectiveRegional(ctx, p.ID); err == nil {
			if loc, err := time.LoadLocation(eff.Timezone); err == nil {
				return loc
			}
		}
	}
	return s.SystemLocation(ctx)
}

// Preferences returns the user's overrides of every user-overridable key ("<key>" → value, or nil = follow the
// system).
func (s *Service) Preferences(ctx context.Context, userID uuid.UUID) (map[string]any, error) {
	rows, err := repo.New(s.db).ListPreferences(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list preferences: %w", err)
	}
	out := map[string]any{}
	for _, d := range s.reg.UserOverridable() {
		out[d.Key] = nil
	}
	for _, r := range rows {
		if d, ok := s.reg.Def(r.Key); ok && d.UserOverridable {
			if v, err := d.decode(r.Value); err == nil {
				out[d.Key] = v
			}
		}
	}
	return out, nil
}

// EffectiveRegional merges the user's overrides over the system values.
func (s *Service) EffectiveRegional(ctx context.Context, userID uuid.UUID) (Regional, map[string]any, error) {
	sys, err := s.SystemRegional(ctx)
	if err != nil {
		return Regional{}, nil, err
	}
	prefs, err := s.Preferences(ctx, userID)
	if err != nil {
		return Regional{}, nil, err
	}
	sysMap := map[string]any{KeyLanguage: sys.Language, KeyTimezone: sys.Timezone, KeyTimeFormat: sys.TimeFormat, KeyDateFormat: sys.DateFormat}
	eff := regionalFrom(func(k string) any {
		if v := prefs[k]; v != nil {
			return v
		}
		return sysMap[k]
	})
	return eff, prefs, nil
}

// UpdatePreferences sets (value) or clears (JSON null = follow the system) the caller's overrides; keys are full
// keys ("regional.timezone"). Each real change is audited (user.preferences.update, old → new).
func (s *Service) UpdatePreferences(ctx context.Context, userID uuid.UUID, changes map[string]json.RawMessage, meta audit.Meta) (map[string]any, error) {
	parsed, err := s.parse("", changes, true)
	if err != nil {
		return nil, err
	}
	var events []audit.Event
	err = s.inTx(ctx, func(q *repo.Queries) error {
		if err := q.LockSettings(ctx, userID.String()); err != nil {
			return err
		}
		rows, err := q.ListPreferences(ctx, userID)
		if err != nil {
			return err
		}
		before := map[string]any{}
		for _, r := range rows {
			if d, ok := s.reg.Def(r.Key); ok {
				if v, err := d.decode(r.Value); err == nil {
					before[r.Key] = v
				}
			}
		}
		for _, c := range parsed {
			old := before[c.def.Key]
			var newValue any
			if c.reset {
				if old == nil {
					continue
				}
				if err := q.DeletePreference(ctx, repo.DeletePreferenceParams{UserID: userID, Key: c.def.Key}); err != nil {
					return err
				}
			} else {
				if reflect.DeepEqual(old, c.value) {
					continue
				}
				if err := q.UpsertPreference(ctx, repo.UpsertPreferenceParams{UserID: userID, Key: c.def.Key, Value: mustJSON(c.value)}); err != nil {
					return err
				}
				newValue = c.value
			}
			events = append(events, audit.Event{Action: ActionPreferencesUpdate, Result: audit.ResultSuccess, TargetType: "user",
				TargetID: userID.String(), Meta: meta, Metadata: map[string]any{"key": c.def.Key, "old": old, "new": newValue}})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, ev := range events {
		_ = s.rec.Record(ctx, ev)
	}
	return s.Preferences(ctx, userID)
}
