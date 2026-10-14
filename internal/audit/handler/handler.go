// Package handler exposes the audit log viewer under /api/v1/audit (v0.3.4): filtered, keyset-paginated events,
// CSV export (rbac.AuditExport, itself audited, formula injection neutralised) and the hash-chain integrity status
// with on-demand verification (rbac.AuditRead).
package handler

import (
	"context"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/agungharyanto/sentra-dcim/internal/audit/chain"
	"github.com/agungharyanto/sentra-dcim/internal/audit/repo"
	audit "github.com/agungharyanto/sentra-dcim/internal/audit/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/rbac"
)

// Stable error codes (translated in the UI: errors:code.<code>).
const (
	CodeValidation = "validation_failed"
	CodeBusy       = "verification_running"
)

// Audit actions of the viewer itself.
const (
	ActionExport = "audit.export"
	ActionVerify = "audit.verify"
)

// Limits.
const (
	DefaultLimit = 50
	MaxLimit     = 200
	MaxExport    = 50000 // rows per CSV export; the response says when it was truncated
	exportBatch  = 1000
)

// Querier is the read side of the audit repository.
type Querier interface {
	ListEvents(ctx context.Context, arg repo.ListEventsParams) ([]repo.ListEventsRow, error)
	ListActions(ctx context.Context) ([]string, error)
}

// Handler serves the audit viewer.
type Handler struct {
	q   Querier
	rec *audit.Recorder
	mon *chain.Monitor
	tz  *time.Location
	log *slog.Logger
}

// New creates a Handler; tz is the display time zone used for the local time column of the CSV export.
func New(q Querier, rec *audit.Recorder, mon *chain.Monitor, tz *time.Location, log *slog.Logger) *Handler {
	if tz == nil {
		tz = time.UTC
	}
	return &Handler{q: q, rec: rec, mon: mon, tz: tz, log: log}
}

// Routes registers the endpoints; require returns the permission middleware (auth handler Require).
func (h *Handler) Routes(r chi.Router, require func(rbac.Permission) func(http.Handler) http.Handler) {
	read := require(rbac.AuditRead)
	r.With(read).Get("/audit/events", h.list)
	r.With(require(rbac.AuditExport)).Get("/audit/events/export.csv", h.export)
	r.With(read).Get("/audit/actions", h.actions)
	r.With(read).Get("/audit/integrity", h.integrity)
	r.With(read).Post("/audit/integrity/verify", h.verify)
}

// Event is one audit record as returned by the API (times in UTC; the UI shows them in the display time zone).
type Event struct {
	Seq           int64           `json:"seq"`
	OccurredAt    time.Time       `json:"occurred_at"`
	ActorType     string          `json:"actor_type"`
	ActorID       *uuid.UUID      `json:"actor_id"`
	ActorUsername string          `json:"actor_username"`
	Action        string          `json:"action"`
	TargetType    string          `json:"target_type"`
	TargetID      string          `json:"target_id"`
	Result        string          `json:"result"`
	IP            string          `json:"ip"`
	UserAgent     string          `json:"user_agent"`
	RequestID     string          `json:"request_id"`
	Metadata      json.RawMessage `json:"metadata"`
	Hash          string          `json:"hash"`
}

type listResponse struct {
	Events     []Event `json:"events"`
	NextBefore *int64  `json:"next_before"` // pass as ?before= for the next (older) page; null on the last page
	Limit      int     `json:"limit"`
}

var (
	actionRe = regexp.MustCompile(`^[a-z][a-z0-9_.]{0,63}$`)
	results  = map[string]bool{"": true, audit.ResultSuccess: true, audit.ResultFailure: true, audit.ResultDenied: true}
)

type fieldErr struct{ field, msg string }

func (e *fieldErr) Error() string { return e.field + ": " + e.msg }

// filter parses the shared query parameters of list and export.
func filter(r *http.Request) (repo.ListEventsParams, map[string]any, error) {
	q := r.URL.Query()
	var p repo.ListEventsParams
	echo := map[string]any{}
	for _, f := range []struct {
		key string
		dst *pgtype.Timestamptz
	}{{"since", &p.Since}, {"until", &p.Until}} {
		if v := q.Get(f.key); v != "" {
			t, err := time.Parse(time.RFC3339, v)
			if err != nil {
				return p, nil, &fieldErr{f.key, "must be an RFC 3339 timestamp"}
			}
			*f.dst = pgtype.Timestamptz{Time: t, Valid: true}
			echo[f.key] = t.UTC().Format(time.RFC3339)
		}
	}
	if p.Since.Valid && p.Until.Valid && !p.Since.Time.Before(p.Until.Time) {
		return p, nil, &fieldErr{"until", "must be after since"}
	}
	p.Actor = strings.TrimSpace(q.Get("actor"))
	p.Action = strings.TrimSuffix(strings.TrimSpace(q.Get("action")), ".")
	p.Result = q.Get("result")
	p.TargetType = strings.TrimSpace(q.Get("target_type"))
	p.TargetID = strings.TrimSpace(q.Get("target_id"))
	switch {
	case utf8.RuneCountInString(p.Actor) > 128:
		return p, nil, &fieldErr{"actor", "at most 128 characters"}
	case p.Action != "" && !actionRe.MatchString(p.Action):
		return p, nil, &fieldErr{"action", "lower-case action name or prefix (e.g. auth or user.create)"}
	case !results[p.Result]:
		return p, nil, &fieldErr{"result", "one of success, failure, denied"}
	case len(p.TargetType) > 64:
		return p, nil, &fieldErr{"target_type", "at most 64 characters"}
	case len(p.TargetID) > 128:
		return p, nil, &fieldErr{"target_id", "at most 128 characters"}
	}
	for k, v := range map[string]string{"actor": p.Actor, "action": p.Action, "result": p.Result, "target_type": p.TargetType,
		"target_id": p.TargetID} {
		if v != "" {
			echo[k] = v
		}
	}
	p.Actor = likeEscape(p.Actor)
	if v := q.Get("before"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return p, nil, &fieldErr{"before", "must be a positive integer"}
		}
		p.Before = pgtype.Int8{Int64: n, Valid: true}
	}
	return p, echo, nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	p, _, err := filter(r)
	if err != nil {
		h.fail(w, r, "list audit events", err)
		return
	}
	limit := DefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > MaxLimit {
			httpserver.FieldError(w, r, http.StatusUnprocessableEntity, CodeValidation, "limit", "between 1 and 200")
			return
		}
		limit = n
	}
	p.Lim = int32(limit + 1) //nolint:gosec // bounded above
	rows, err := h.q.ListEvents(r.Context(), p)
	if err != nil {
		h.fail(w, r, "list audit events", err)
		return
	}
	resp := listResponse{Events: make([]Event, 0, min(len(rows), limit)), Limit: limit}
	for i, row := range rows {
		if i == limit {
			next := rows[i-1].ChainSeq
			resp.NextBefore = &next
			break
		}
		resp.Events = append(resp.Events, toEvent(row))
	}
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, http.StatusOK, resp)
}

func toEvent(row repo.ListEventsRow) Event {
	md := json.RawMessage(row.Metadata)
	if len(md) == 0 {
		md = json.RawMessage("{}")
	}
	return Event{Seq: row.ChainSeq, OccurredAt: row.OccurredAt.Time.UTC(), ActorType: row.ActorType, ActorID: row.ActorID,
		ActorUsername: row.ActorUsername, Action: row.Action, TargetType: row.TargetType, TargetID: row.TargetID,
		Result: row.Result, IP: row.Ip, UserAgent: row.UserAgent, RequestID: row.RequestID, Metadata: md,
		Hash: hex.EncodeToString(row.Hash)}
}

// CSVColumns is the header of the export (stable; documented in the Help Center).
var CSVColumns = []string{"seq", "occurred_at_utc", "occurred_at_local", "actor_type", "actor_username", "actor_id",
	"action", "result", "target_type", "target_id", "ip", "user_agent", "request_id", "metadata", "hash"}

// SafeCell neutralises spreadsheet formula injection (CWE-1236): a cell starting with = + - @ (or a tab/carriage
// return that some spreadsheets strip first) is prefixed with a single quote so it is shown as text.
func SafeCell(s string) string {
	if s == "" {
		return s
	}
	switch s[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + s
	}
	return s
}

func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	p, echo, err := filter(r)
	if err != nil {
		h.fail(w, r, "export audit events", err)
		return
	}
	if p.Before.Valid {
		echo["before"] = p.Before.Int64
	}
	now := time.Now().UTC()
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="sentra-audit-`+now.In(h.tz).Format("20060102-150405")+`.csv"`)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	cw := csv.NewWriter(w)
	_ = cw.Write(CSVColumns)
	rows, truncated, err := h.writeCSV(r.Context(), cw, p)
	cw.Flush()
	result := audit.ResultSuccess
	md := map[string]any{"rows": rows, "truncated": truncated, "filters": echo}
	if err == nil {
		err = cw.Error()
	}
	if err != nil {
		result = audit.ResultFailure
		md["error"] = "export interrupted"
		h.log.ErrorContext(r.Context(), "export audit events", "error", err)
	}
	// Recorded after the export so it carries the row count; it is never part of its own export.
	_ = h.rec.Record(context.WithoutCancel(r.Context()), audit.Event{Action: ActionExport, Result: result,
		TargetType: "audit_log", Meta: audit.MetaFromRequest(r), Metadata: md})
}

func (h *Handler) writeCSV(ctx context.Context, cw *csv.Writer, p repo.ListEventsParams) (int, bool, error) {
	n := 0
	for {
		p.Lim = exportBatch
		rows, err := h.q.ListEvents(ctx, p)
		if err != nil {
			return n, false, err
		}
		for _, row := range rows {
			if n == MaxExport {
				return n, true, nil
			}
			e := toEvent(row)
			actorID := ""
			if e.ActorID != nil {
				actorID = e.ActorID.String()
			}
			rec := []string{strconv.FormatInt(e.Seq, 10), e.OccurredAt.Format(time.RFC3339Nano),
				e.OccurredAt.In(h.tz).Format("2006-01-02 15:04:05 MST"), e.ActorType, e.ActorUsername, actorID, e.Action,
				e.Result, e.TargetType, e.TargetID, e.IP, e.UserAgent, e.RequestID, string(e.Metadata), e.Hash}
			for i := range rec {
				rec[i] = SafeCell(rec[i])
			}
			if err := cw.Write(rec); err != nil {
				return n, false, err
			}
			n++
		}
		if len(rows) < exportBatch {
			return n, false, nil
		}
		p.Before = pgtype.Int8{Int64: rows[len(rows)-1].ChainSeq, Valid: true}
		cw.Flush()
	}
}

func (h *Handler) actions(w http.ResponseWriter, r *http.Request) {
	acts, err := h.q.ListActions(r.Context())
	if err != nil {
		h.fail(w, r, "list audit actions", err)
		return
	}
	if acts == nil {
		acts = []string{}
	}
	httpserver.JSON(w, http.StatusOK, map[string]any{"actions": acts})
}

type integrityResponse struct {
	Last    *chain.Result `json:"last"` // null before the first verification since the app started
	Running bool          `json:"running"`
}

func (h *Handler) integrity(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	httpserver.JSON(w, http.StatusOK, integrityResponse{Last: h.mon.Last(), Running: h.mon.Running()})
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	res, err := h.mon.VerifyNow(context.WithoutCancel(r.Context()))
	if errors.Is(err, chain.ErrBusy) {
		httpserver.Error(w, r, http.StatusConflict, CodeBusy, "a verification is already running")
		return
	}
	if err != nil {
		h.fail(w, r, "verify audit chain", err)
		return
	}
	result := audit.ResultSuccess
	md := map[string]any{"ok": res.OK, "checked": res.Checked, "head_seq": res.HeadSeq}
	if !res.OK {
		result = audit.ResultFailure
		md["broken_seq"] = res.BrokenSeq
	}
	_ = h.rec.Record(r.Context(), audit.Event{Action: ActionVerify, Result: result, TargetType: "audit_log",
		Meta: audit.MetaFromRequest(r), Metadata: md})
	httpserver.JSON(w, http.StatusOK, res)
}

func (h *Handler) fail(w http.ResponseWriter, r *http.Request, what string, err error) {
	var fe *fieldErr
	if errors.As(err, &fe) {
		httpserver.FieldError(w, r, http.StatusUnprocessableEntity, CodeValidation, fe.field, fe.msg)
		return
	}
	h.log.ErrorContext(r.Context(), what, "error", err)
	httpserver.Error(w, r, http.StatusInternalServerError, "internal_error", "internal server error")
}

// likeEscape escapes LIKE wildcards so the actor filter is literal (the query uses ESCAPE '\').
func likeEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
