package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/agungharyanto/sentra-dcim/internal/audit/repo"
	"github.com/agungharyanto/sentra-dcim/internal/platform/principal"
)

type fakeQ struct{ got []repo.InsertEventParams }

func (f *fakeQ) InsertEvent(_ context.Context, a repo.InsertEventParams) (repo.InsertEventRow, error) {
	f.got = append(f.got, a)
	return repo.InsertEventRow{ID: int64(len(f.got))}, nil
}

func TestSanitizeDropsCredentials(t *testing.T) {
	in := map[string]any{"reason": "invalid", "Password": "x", "new_password": "y", "session_token": "z",
		"password_hash": "h", "X-CSRF": "c", "client_secret": "s", "count": 3}
	out := Sanitize(in)
	if len(out) != 2 || out["reason"] != "invalid" || out["count"] != 3 {
		t.Fatalf("sanitize = %v", out)
	}
}

func TestRecordUsesPrincipalAndMeta(t *testing.T) {
	f := &fakeQ{}
	r := New(f, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.RemoteAddr = "203.0.113.7:4444"
	req.Header.Set("User-Agent", "test-agent")
	id := uuid.New()
	ctx := principal.With(context.Background(), principal.Principal{Type: principal.TypeUser, ID: id, Username: "nina"})
	if err := r.Record(ctx, Event{Action: "auth.logout", Result: ResultSuccess, Meta: MetaFromRequest(req),
		Metadata: map[string]any{"token": "secret-value", "ok": true}}); err != nil {
		t.Fatal(err)
	}
	got := f.got[0]
	if got.ActorType != ActorUser || got.ActorID == nil || *got.ActorID != id || got.ActorUsername != "nina" {
		t.Fatalf("actor = %+v", got)
	}
	if got.Ip == nil || got.Ip.String() != "203.0.113.7" || got.UserAgent != "test-agent" {
		t.Fatalf("meta = %+v", got)
	}
	var md map[string]any
	_ = json.Unmarshal(got.Metadata, &md)
	if _, leaked := md["token"]; leaked || md["ok"] != true {
		t.Fatalf("metadata = %s", got.Metadata)
	}
}

func TestRecordAnonymousAndSystem(t *testing.T) {
	f := &fakeQ{}
	r := New(f, slog.New(slog.NewTextHandler(io.Discard, nil)))
	_ = r.Record(context.Background(), Event{Action: "auth.login.failure", Result: ResultFailure, ActorUsername: "ghost"})
	_ = r.Record(context.Background(), Event{Action: "user.create_admin", Result: ResultSuccess, ActorType: ActorSystem, ActorUsername: "cli"})
	if f.got[0].ActorType != ActorAnonymous || f.got[0].ActorUsername != "ghost" || f.got[0].ActorID != nil {
		t.Fatalf("anonymous = %+v", f.got[0])
	}
	if f.got[1].ActorType != ActorSystem || f.got[1].ActorUsername != "cli" {
		t.Fatalf("system = %+v", f.got[1])
	}
}

func TestSanitizeNestedAndMFA(t *testing.T) {
	out := Sanitize(map[string]any{"totp": "123456", "OTP_code": "1", "recovery_code": "abcd", "recovery_codes_remaining": 7,
		"nested": map[string]any{"secret": "s", "keep": 1, "list": []any{map[string]any{"api_key": "k", "n": 2}}}})
	b, _ := json.Marshal(out)
	if string(b) != `{"nested":{"keep":1,"list":[{"n":2}]},"recovery_codes_remaining":7}` {
		t.Fatalf("sanitize = %s", b)
	}
}

func TestThrottleAggregatesRepeats(t *testing.T) {
	f := &fakeQ{}
	r := New(f, slog.New(slog.NewTextHandler(io.Discard, nil)))
	now := time.Date(2026, 10, 14, 10, 0, 0, 0, time.UTC)
	r.throttle.now = func() time.Time { return now }
	ev := Event{Action: "authz.denied", Result: ResultDenied, TargetID: "GET /x", Metadata: map[string]any{"permission": "p"}}
	for i := 0; i < 50; i++ {
		r.RecordAggregated(context.Background(), "u1|GET /x", ev)
	}
	r.RecordAggregated(context.Background(), "u2|GET /x", ev) // another actor: own first event
	if len(f.got) != 2 {
		t.Fatalf("recorded %d events, want 2 (first per key)", len(f.got))
	}
	now = now.Add(DefaultThrottleWindow + time.Second)
	r.throttle.flush(context.Background(), false)
	if len(f.got) != 3 || f.got[2].Action != "authz.denied.repeated" {
		t.Fatalf("summary missing: %+v", f.got)
	}
	var md map[string]any
	_ = json.Unmarshal(f.got[2].Metadata, &md)
	if md["repeated"] != float64(49) || md["permission"] != "p" {
		t.Fatalf("summary metadata = %s", f.got[2].Metadata)
	}
	// Window closed and flushed: the next one is recorded again.
	r.RecordAggregated(context.Background(), "u1|GET /x", ev)
	if len(f.got) != 4 || f.got[3].Action != "authz.denied" {
		t.Fatalf("after window: %+v", f.got)
	}
}

func TestThrottleBoundsKeys(t *testing.T) {
	f := &fakeQ{}
	r := New(f, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r.throttle.max = 3
	ev := Event{Action: "authz.denied", Result: ResultDenied}
	for i := 0; i < 100; i++ {
		r.RecordAggregated(context.Background(), fmt.Sprintf("ip-%d", i), ev)
	}
	if len(r.throttle.m) > 4 || len(f.got) != 4 { // 3 keys + 1 overflow bucket
		t.Fatalf("keys=%d events=%d", len(r.throttle.m), len(f.got))
	}
	r.throttle.flush(context.Background(), true)
	if last := f.got[len(f.got)-1]; last.Action != "audit.throttle_overflow" || last.ActorType != ActorSystem {
		t.Fatalf("overflow summary = %+v", last)
	}
}
