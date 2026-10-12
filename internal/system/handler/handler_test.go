package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/agungharyanto/sentra-dcim/internal/platform/buildinfo"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	"github.com/agungharyanto/sentra-dcim/internal/system/handler"
	"github.com/agungharyanto/sentra-dcim/internal/system/service"
)

func newServer(t *testing.T, checkers ...service.Checker) *httptest.Server {
	t.Helper()
	loc, _ := time.LoadLocation("Asia/Makassar")
	svc := service.New(logging.Discard(), loc, checkers...)
	r := httpserver.NewRouter(logging.Discard(), nil, nil, func(r chi.Router) { handler.New(svc).Routes(r) })
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

type result struct {
	StatusCode int
	Header     http.Header
}

func get(t *testing.T, url string, out any) result {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("decode: %v", err)
		}
	}
	return result{StatusCode: resp.StatusCode, Header: resp.Header}
}

var okCheck = service.FuncChecker{CheckName: "database", Fn: func(context.Context) error { return nil }}

func TestHealth(t *testing.T) {
	srv := newServer(t)
	var body map[string]string
	resp := get(t, srv.URL+"/api/v1/health", &body)
	if resp.StatusCode != http.StatusOK || body["status"] != "ok" {
		t.Fatalf("got %d %v", resp.StatusCode, body)
	}
	if !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		t.Fatalf("content-type = %s", resp.Header.Get("Content-Type"))
	}
	for _, h := range []string{"X-Content-Type-Options", "X-Frame-Options", "Content-Security-Policy", "X-Request-Id"} {
		if resp.Header.Get(h) == "" {
			t.Errorf("missing header %s", h)
		}
	}
}

func TestReadyOK(t *testing.T) {
	srv := newServer(t, okCheck)
	var rep service.Readiness
	resp := get(t, srv.URL+"/api/v1/ready", &rep)
	if resp.StatusCode != http.StatusOK || rep.Status != "ready" || rep.Checks["database"].Status != "ok" {
		t.Fatalf("got %d %+v", resp.StatusCode, rep)
	}
}

func TestReadyUnavailable(t *testing.T) {
	bad := service.FuncChecker{CheckName: "redis", Fn: func(context.Context) error { return errors.New("secret-host:6379 refused") }}
	srv := newServer(t, okCheck, bad)
	resp, err := http.Get(srv.URL + "/api/v1/ready")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var raw strings.Builder
	var rep service.Readiness
	if err := json.NewDecoder(teeReader{resp, &raw}).Decode(&rep); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable || rep.Status != "not_ready" {
		t.Fatalf("got %d %+v", resp.StatusCode, rep)
	}
	if strings.Contains(raw.String(), "secret-host") {
		t.Fatal("internal error details leaked to client")
	}
}

func TestVersion(t *testing.T) {
	buildinfo.Version = "v0.1.3-test"
	srv := newServer(t)
	var v map[string]any
	resp := get(t, srv.URL+"/api/v1/version", &v)
	if resp.StatusCode != http.StatusOK || v["version"] != "v0.1.3-test" || v["display_timezone"] != "Asia/Makassar" {
		t.Fatalf("got %d %v", resp.StatusCode, v)
	}
	for _, k := range []string{"commit", "go_version", "server_time_utc", "server_time_display"} {
		if _, ok := v[k]; !ok {
			t.Errorf("missing field %s", k)
		}
	}
}

func TestNotFoundAndMethodNotAllowed(t *testing.T) {
	srv := newServer(t)
	var e httpserver.ErrorBody
	if resp := get(t, srv.URL+"/api/v1/nope", &e); resp.StatusCode != http.StatusNotFound || e.Error.Code != "not_found" || e.Error.RequestID == "" {
		t.Fatalf("404: %d %+v", resp.StatusCode, e)
	}
	resp, err := http.Post(srv.URL+"/api/v1/health", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("405: got %d", resp.StatusCode)
	}
}

type teeReader struct {
	resp *http.Response
	sb   *strings.Builder
}

func (t teeReader) Read(p []byte) (int, error) {
	n, err := t.resp.Body.Read(p)
	t.sb.Write(p[:n])
	return n, err
}
