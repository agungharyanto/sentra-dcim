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

// Recorder writes audit events. Failures are logged and never break the caller's request.
type Recorder struct {
	q   repo.Querier
	log *slog.Logger
}

// New creates a Recorder.
func New(q repo.Querier, log *slog.Logger) *Recorder { return &Recorder{q: q, log: log} }

var sensitive = []string{"password", "passwd", "token", "hash", "secret", "cookie", "authorization", "csrf"}

// Sanitize drops metadata keys that could carry credentials (case-insensitive substring match).
func Sanitize(md map[string]any) map[string]any {
	out := make(map[string]any, len(md))
	for k, v := range md {
		lk := strings.ToLower(k)
		drop := false
		for _, s := range sensitive {
			if strings.Contains(lk, s) {
				drop = true
				break
			}
		}
		if !drop {
			out[k] = v
		}
	}
	return out
}

// Record stores ev. It returns the error for tests; callers usually ignore it (it is logged).
func (r *Recorder) Record(ctx context.Context, ev Event) error {
	actorType, actorID, actorName := ActorAnonymous, ev.ActorID, ev.ActorUsername
	if p, ok := principal.From(ctx); ok {
		actorType = ActorUser
		if actorID == nil {
			id := p.ID
			actorID = &id
		}
		if actorName == "" {
			actorName = p.Username
		}
	}
	if ev.ActorType != "" {
		actorType = ev.ActorType
	}
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
