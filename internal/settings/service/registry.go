// Package service implements the generic settings store (v0.3.5, D-076): a reviewed registry of keys in code (type,
// default, allowed values, min/max, validator, user-overridable), values in the database only when they differ from
// the default, per-user overrides for user-overridable keys, and an audit event (old → new) for every change.
// Regional settings (v0.3.5) are the first group; Allowed IP (v0.3.6) and session/password policy (v0.3.7) add
// their keys here.
package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Types of setting values.
const (
	TypeEnum   = "enum"
	TypeString = "string"
	TypeInt    = "int"
	TypeBool   = "bool"
)

// Def is the schema of one key. Values travel as JSON: a string for enum/string, a number for int, true/false for
// bool.
type Def struct {
	Key             string   `json:"key"` // "<group>.<name>"
	Type            string   `json:"type"`
	Enum            []string `json:"enum,omitempty"`
	Min             *int64   `json:"min,omitempty"`
	Max             *int64   `json:"max,omitempty"`
	MaxLen          int      `json:"max_length,omitempty"`
	UserOverridable bool     `json:"user_overridable"`
	// Managed keys are shown by GET /settings/{group} but changed only through their own endpoint (with its own
	// guards, e.g. the Allowed IP lockout protection); the generic PUT refuses them.
	Managed bool `json:"managed"`
	// Default is filled when the registry is built (some defaults come from .env).
	Default any `json:"default"`
	// Validate adds a check beyond type/enum/range (e.g. a known IANA time zone).
	Validate func(v any) error `json:"-"`
}

// Group returns "<group>" of "<group>.<name>".
func (d Def) Group() string { g, _, _ := strings.Cut(d.Key, "."); return g }

// Name returns "<name>" of "<group>.<name>".
func (d Def) Name() string { _, n, _ := strings.Cut(d.Key, "."); return n }

// Regional keys (v0.3.5).
const (
	KeyLanguage   = "regional.language"
	KeyTimezone   = "regional.timezone"
	KeyTimeFormat = "regional.time_format"
	KeyDateFormat = "regional.date_format"
)

// Allowed IP keys (v0.3.6, D-078), managed by internal/allowlist.
const (
	KeyAllowlistMode    = "allowlist.mode"
	KeyAllowlistPending = "allowlist.pending_until" // RFC 3339 deadline of the auto-rollback to monitor, "" = none
)

// AllowlistModes are the values of allowlist.mode.
var AllowlistModes = []string{"off", "monitor", "enforce"}

// Allowed regional values.
var (
	Languages   = []string{"id", "en"}
	TimeFormats = []string{"24h", "12h"}
	DateFormats = []string{"DD/MM/YYYY", "YYYY-MM-DD", "DD MMM YYYY"}
)

// Defaults are the values used when no setting is stored (from .env: DEFAULT_LOCALE, DISPLAY_TIMEZONE,
// AUTH_SESSION_IDLE, AUTH_SESSION_ABSOLUTE). Zero session durations mean 30 min / 12 h.
type Defaults struct {
	Language        string
	Timezone        string
	SessionIdle     time.Duration
	SessionAbsolute time.Duration
}

// Session and password policy keys (v0.3.7, D-069/D-070/D-079).
const (
	KeySessionIdleMinutes   = "session.idle_minutes"
	KeySessionAbsoluteHours = "session.absolute_hours"
	KeyPasswordExpiryDays   = "password.expiry_days"
	KeyPasswordWarnDays     = "password.warn_days"
	KeyPasswordHistory      = "password.history"
)

func i64(n int64) *int64 { return &n }

// clamp keeps a .env default inside the setting's range.
func clamp(v, lo, hi int64) int64 { return max(lo, min(hi, v)) }

// ValidTimezone accepts IANA zone names ("Asia/Jakarta", "UTC"); rejects "Local", empty and unknown names.
func ValidTimezone(name string) error {
	if name == "" || name == "Local" || len(name) > 64 || strings.ContainsAny(name, " \\") || strings.Contains(name, "..") {
		return errors.New("unknown time zone")
	}
	if _, err := time.LoadLocation(name); err != nil {
		return errors.New("unknown time zone")
	}
	return nil
}

// NewRegistry builds the reviewed key registry. Every key added here needs a validation test and documentation.
func NewRegistry(d Defaults) (*Registry, error) {
	if !slices.Contains(Languages, d.Language) {
		return nil, fmt.Errorf("default language %q not supported", d.Language)
	}
	if err := ValidTimezone(d.Timezone); err != nil {
		return nil, fmt.Errorf("default time zone %q: %w", d.Timezone, err)
	}
	idle, abs := d.SessionIdle, d.SessionAbsolute
	if idle <= 0 {
		idle = 30 * time.Minute
	}
	if abs <= 0 {
		abs = 12 * time.Hour
	}
	defs := []Def{
		{Key: KeyLanguage, Type: TypeEnum, Enum: Languages, Default: d.Language, UserOverridable: true},
		{Key: KeyTimezone, Type: TypeString, MaxLen: 64, Default: d.Timezone, UserOverridable: true,
			Validate: func(v any) error { return ValidTimezone(v.(string)) }},
		{Key: KeyTimeFormat, Type: TypeEnum, Enum: TimeFormats, Default: "24h", UserOverridable: true},
		{Key: KeyDateFormat, Type: TypeEnum, Enum: DateFormats, Default: "DD/MM/YYYY", UserOverridable: true},
		{Key: KeySessionIdleMinutes, Type: TypeInt, Min: i64(5), Max: i64(240), Default: clamp(int64(idle/time.Minute), 5, 240)},
		{Key: KeySessionAbsoluteHours, Type: TypeInt, Min: i64(1), Max: i64(24), Default: clamp(int64(abs/time.Hour), 1, 24)},
		{Key: KeyPasswordExpiryDays, Type: TypeInt, Min: i64(0), Max: i64(365), Default: int64(90),
			Validate: func(v any) error {
				if n := v.(int64); n != 0 && n < 30 {
					return errors.New("0 (never) or between 30 and 365")
				}
				return nil
			}},
		{Key: KeyPasswordWarnDays, Type: TypeInt, Min: i64(1), Max: i64(60), Default: int64(14)},
		{Key: KeyPasswordHistory, Type: TypeInt, Min: i64(1), Max: i64(24), Default: int64(5)},
		{Key: KeyAllowlistMode, Type: TypeEnum, Enum: AllowlistModes, Default: "off", Managed: true},
		{Key: KeyAllowlistPending, Type: TypeString, MaxLen: 40, Default: "", Managed: true,
			Validate: func(v any) error {
				if s := v.(string); s != "" {
					if _, err := time.Parse(time.RFC3339, s); err != nil {
						return errors.New("must be an RFC 3339 time or empty")
					}
				}
				return nil
			}},
	}
	r := &Registry{byKey: map[string]Def{}}
	for _, def := range defs {
		if _, err := def.decode(mustJSON(def.Default)); err != nil {
			return nil, fmt.Errorf("default of %s: %w", def.Key, err)
		}
		r.byKey[def.Key] = def
		r.order = append(r.order, def.Key)
	}
	return r, nil
}

// Registry is the set of known keys.
type Registry struct {
	byKey map[string]Def
	order []string
}

// CheckGroup validates rules that span several keys of a group, on the values a change would leave in place
// (name → value). It returns the offending name and reason.
func (r *Registry) CheckGroup(group string, values map[string]any) (string, string) {
	switch group {
	case "session":
		idle, _ := values["idle_minutes"].(int64)
		abs, _ := values["absolute_hours"].(int64)
		if idle > abs*60 {
			return "idle_minutes", "must not exceed the absolute session limit"
		}
	case "password":
		exp, _ := values["expiry_days"].(int64)
		warn, _ := values["warn_days"].(int64)
		if exp != 0 && warn >= exp {
			return "warn_days", "must be shorter than the expiry period"
		}
	}
	return "", ""
}

// Def returns the schema of key.
func (r *Registry) Def(key string) (Def, bool) { d, ok := r.byKey[key]; return d, ok }

// Group returns the keys of a group in registration order (nil for an unknown group).
func (r *Registry) Group(group string) []Def {
	var out []Def
	for _, k := range r.order {
		if d := r.byKey[k]; d.Group() == group {
			out = append(out, d)
		}
	}
	return out
}

// UserOverridable returns the keys users may override, in registration order.
func (r *Registry) UserOverridable() []Def {
	var out []Def
	for _, k := range r.order {
		if d := r.byKey[k]; d.UserOverridable {
			out = append(out, d)
		}
	}
	return out
}

func mustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// decode parses and validates a JSON value for this key and returns the canonical Go value.
func (d Def) decode(raw json.RawMessage) (any, error) {
	switch d.Type {
	case TypeEnum, TypeString:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, errors.New("must be a string")
		}
		if d.Type == TypeEnum && !slices.Contains(d.Enum, s) {
			return nil, fmt.Errorf("must be one of %s", strings.Join(d.Enum, ", "))
		}
		if d.MaxLen > 0 && len(s) > d.MaxLen {
			return nil, fmt.Errorf("at most %d characters", d.MaxLen)
		}
		return d.check(s)
	case TypeInt:
		var n int64
		if err := json.Unmarshal(raw, &n); err != nil {
			return nil, errors.New("must be an integer")
		}
		if d.Min != nil && n < *d.Min {
			return nil, fmt.Errorf("must be at least %d", *d.Min)
		}
		if d.Max != nil && n > *d.Max {
			return nil, fmt.Errorf("must be at most %d", *d.Max)
		}
		return d.check(n)
	case TypeBool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, errors.New("must be true or false")
		}
		return d.check(b)
	}
	return nil, fmt.Errorf("unsupported type %q", d.Type)
}

func (d Def) check(v any) (any, error) {
	if d.Validate != nil {
		if err := d.Validate(v); err != nil {
			return nil, err
		}
	}
	return v, nil
}
