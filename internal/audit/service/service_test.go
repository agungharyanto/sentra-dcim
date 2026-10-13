package service

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

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
