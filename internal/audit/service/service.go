// Package service records security-relevant events into the append-only audit_events table (ADR-003, D-051).
// Never pass passwords, tokens or hashes: metadata keys that look sensitive are dropped as a last line of defence.
package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/agungharyanto/sentra-dcim/internal/audit/repo"
	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
)

// Results.
const (
	ResultSuccess = "success"
	ResultFailure = "failure"
	ResultDenied  = "denied"
)

// Actor types.
const (
	ActorUser      = "user"
	ActorSystem    = "system"
	ActorAnonymous = "anonymous"
)

// Meta describes where a request came from.
type Meta struct {
	IP        *netip.Addr
	UserAgent string
	RequestID string
}

// MetaFromRequest extracts client IP (already resolved by the realIP middleware), user agent and request ID.
func MetaFromRequest(r *http.Request) Meta {
	m := Meta{UserAgent: truncate(r.UserAgent(), 512), RequestID: middleware.GetReqID(r.Context())}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		ip = ip.Unmap()
		m.IP = &ip
	}
	return m
}

// Event is one audit record. Actor fields are taken from the request principal unless set explicitly.
type Event struct {
	Action     string // e.g. auth.login.success
	Result     string
	TargetType string
	TargetID   string
	Metadata   map[string]any
	Meta       Meta

	ActorType     string     // optional override (system/anonymous)
	ActorID       *uuid.UUID // optional override
	ActorUsername string     // optional override (e.g. the attempted username of a failed login)
}

// Inserter stores one event (repo.Queries).
type Inserter interface {
	InsertEvent(ctx context.Context, arg repo.InsertEventParams) (repo.InsertEventRow, error)
}

// Recorder writes audit events. Failures are logged and never break the caller's request.
type Recorder struct {
	q        Inserter
	log      *slog.Logger
	throttle *throttle
}

// New creates a Recorder.
func New(q Inserter, log *slog.Logger) *Recorder {
	r := &Recorder{q: q, log: log}
	r.throttle = newThrottle(r, DefaultThrottleWindow, defaultThrottleKeys)
	return r
}

// sensitive key fragments (case-insensitive substring match). "otp" covers totp/hotp codes; "recovery_code" the
// MFA recovery codes ("recovery_codes_remaining" is a count and is kept by the exception below).
var sensitive = []string{"password", "passwd", "token", "hash", "secret", "cookie", "authorization", "csrf", "otp",
	"recovery_code", "credential", "private", "api_key", "apikey", "enc_key", "session_key", "passphrase", "pin_code"}

// allowedCounts are harmless counters whose names contain a sensitive fragment.
var allowedCounts = map[string]bool{"recovery_codes_remaining": true}

// Sanitize drops metadata keys that could carry credentials, recursively in nested objects and lists.
func Sanitize(md map[string]any) map[string]any {
	out := make(map[string]any, len(md))
	for k, v := range md {
		if sensitiveKey(k) {
			continue
		}
		out[k] = sanitizeValue(v)
	}
	return out
}

func sensitiveKey(k string) bool {
	lk := strings.ToLower(k)
	if allowedCounts[lk] {
		return false
	}
	for _, s := range sensitive {
		if strings.Contains(lk, s) {
			return true
		}
	}
	return false
}

func sanitizeValue(v any) any {
	switch t := v.(type) {
	case map[string]any:
		return Sanitize(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = sanitizeValue(e)
		}
		return out
	}
	return v
}

// Record stores ev. It returns the error for tests; callers usually ignore it (it is logged).
func (r *Recorder) Record(ctx context.Context, ev Event) error {
	return r.insert(ctx, resolve(ctx, ev))
}

// resolve fixes the actor from the request principal (for events recorded later, outside the request).
func resolve(ctx context.Context, ev Event) Event {
	if p, ok := principal.From(ctx); ok {
		if ev.ActorType == "" {
			ev.ActorType = ActorUser
		}
		if ev.ActorID == nil {
			id := p.ID
			ev.ActorID = &id
		}
		if ev.ActorUsername == "" {
			ev.ActorUsername = p.Username
		}
	}
	if ev.ActorType == "" {
		ev.ActorType = ActorAnonymous
	}
	return ev
}

func (r *Recorder) insert(ctx context.Context, ev Event) error {
	actorType, actorID, actorName := ev.ActorType, ev.ActorID, ev.ActorUsername
	md, err := json.Marshal(Sanitize(ev.Metadata))
	if err != nil {
		md = []byte("{}")
	}
	_, err = r.q.InsertEvent(ctx, repo.InsertEventParams{
		ActorType:     actorType,
		ActorID:       actorID,
		ActorUsername: truncate(actorName, 128),
		Action:        ev.Action,
		TargetType:    ev.TargetType,
		TargetID:      ev.TargetID,
		Result:        ev.Result,
		Ip:            ev.Meta.IP,
		UserAgent:     ev.Meta.UserAgent,
		RequestID:     truncate(ev.Meta.RequestID, 128),
		Metadata:      md,
	})
	if err != nil {
		r.log.ErrorContext(ctx, "audit record failed", "action", ev.Action, "error", err)
	}
	return err
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	// Cut on a rune boundary.
	for n > 0 && !isRuneStart(s[n]) {
		n--
	}
	return s[:n]
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
