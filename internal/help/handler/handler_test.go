package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/go-chi/chi/v5"

	apispec "github.com/agungharyanto/sentra-dcim/api"
	helpdocs "github.com/agungharyanto/sentra-dcim/docs/help"
	"github.com/agungharyanto/sentra-dcim/internal/help/handler"
	"github.com/agungharyanto/sentra-dcim/internal/help/service"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	systemhandler "github.com/agungharyanto/sentra-dcim/internal/system/handler"
	systemservice "github.com/agungharyanto/sentra-dcim/internal/system/service"
	uiservice "github.com/agungharyanto/sentra-dcim/internal/ui/service"
)

func menus() []service.Menu {
	_, ms := uiservice.Registry()
	out := []service.Menu{}
	for _, m := range ms {
		out = append(out, service.Menu{Key: m.Key, Path: m.Path, Available: m.Status == uiservice.StatusAvailable})
	}
	return out
}

func newServer(t *testing.T, cat *service.Catalog) *httptest.Server {
	t.Helper()
	loc, _ := time.LoadLocation("Asia/Jakarta")
	sys := systemservice.New(logging.Discard(), loc)
	r := httpserver.NewRouter(logging.Discard(), nil, nil, func(r chi.Router) {
		systemhandler.New(sys).Routes(r)
		handler.New(cat, "id", []string{"id", "en"}).Routes(r)
	})
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

func realCatalog(t *testing.T) *service.Catalog {
	t.Helper()
	cat, err := service.Load(helpdocs.FS, []string{"id", "en"}, menus(), apispec.OpenAPI)
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestIndexContract(t *testing.T) {
	srv := newServer(t, realCatalog(t))
	var doc map[string]any
	resp := get(t, srv.URL+"/api/v1/help/index?locale=en", &doc)
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("status=%d cache=%q", resp.StatusCode, resp.Header.Get("Cache-Control"))
	}
	if len(doc) != 2 || doc["locale"] != "en" {
		t.Fatalf("doc = %v", doc)
	}
	pages := doc["pages"].([]any)
	if len(pages) == 0 {
		t.Fatal("no pages")
	}
	for _, p := range pages {
		m := p.(map[string]any)
		for _, k := range []string{"slug", "title", "audience", "order", "headings"} {
			if _, ok := m[k]; !ok {
				t.Fatalf("index entry %v lacks %q", m, k)
			}
		}
		if _, ok := m["markdown"]; ok {
			t.Fatalf("index must not include page bodies: %v", m)
		}
	}
	// Default locale when the parameter is missing.
	get(t, srv.URL+"/api/v1/help/index", &doc)
	if doc["locale"] != "id" {
		t.Fatalf("default locale = %v", doc["locale"])
	}
}

func TestPage(t *testing.T) {
	srv := newServer(t, realCatalog(t))
	var p map[string]any
	resp := get(t, srv.URL+"/api/v1/help/pages/dashboard?locale=en", &p)
	if resp.StatusCode != http.StatusOK || p["slug"] != "dashboard" || p["locale"] != "en" || p["menu"] != "dashboard" || p["markdown"] == "" {
		t.Fatalf("status=%d page=%v", resp.StatusCode, p)
	}
}

func TestPageErrors(t *testing.T) {
	srv := newServer(t, realCatalog(t))
	cases := []struct {
		path string
		code int
		err  string
	}{
		{"/api/v1/help/pages/does-not-exist", 404, "not_found"},
		{"/api/v1/help/pages/..%2f..%2fREADME", 404, "not_found"},
		{"/api/v1/help/pages/..%2Fid%2Fdashboard", 404, "not_found"},
		{"/api/v1/help/pages/dashboard%2F..%2Fhelp", 404, "not_found"},
		{"/api/v1/help/pages/%2e%2e/etc/passwd", 404, "not_found"},
		{"/api/v1/help/pages/dashboard/index", 404, "not_found"},
		{"/api/v1/help/pages/dashboard?locale=fr", 400, "invalid_locale"},
		{"/api/v1/help/index?locale=../id", 400, "invalid_locale"},
	}
	for _, tc := range cases {
		var body struct {
			Error struct{ Code string } `json:"error"`
		}
		resp := get(t, srv.URL+tc.path, &body)
		if resp.StatusCode != tc.code || body.Error.Code != tc.err {
			t.Errorf("%s: status=%d code=%q, want %d %q", tc.path, resp.StatusCode, body.Error.Code, tc.code, tc.err)
		}
	}
}

func TestOpenAPI(t *testing.T) {
	srv := newServer(t, realCatalog(t))
	var spec map[string]any
	resp := get(t, srv.URL+"/api/v1/help/openapi", &spec)
	if resp.StatusCode != http.StatusOK || spec["openapi"] != "3.1.0" {
		t.Fatalf("status=%d openapi=%v", resp.StatusCode, spec["openapi"])
	}
	paths := spec["paths"].(map[string]any)
	for _, p := range []string{"/api/v1/health", "/api/v1/ui/bootstrap", "/api/v1/help/index", "/api/v1/help/pages/{slug}"} {
		if _, ok := paths[p]; !ok {
			t.Errorf("openapi lacks %s", p)
		}
	}
}

// An invalid catalog (Load error → nil) disables only help: 503 + ErrorBody, health keeps answering.
func TestDisabledModule(t *testing.T) {
	_, err := service.Load(fstest.MapFS{"id/x/index.md": {Data: []byte("broken")}}, []string{"id", "en"}, menus(), apispec.OpenAPI)
	if err == nil {
		t.Fatal("expected invalid catalog")
	}
	srv := newServer(t, nil)
	for _, p := range []string{"/api/v1/help/index", "/api/v1/help/pages/dashboard", "/api/v1/help/openapi"} {
		var body struct {
			Error struct{ Code, Message string } `json:"error"`
		}
		resp := get(t, srv.URL+p, &body)
		if resp.StatusCode != http.StatusServiceUnavailable || body.Error.Code != "help_unavailable" {
			t.Errorf("%s: status=%d body=%+v", p, resp.StatusCode, body)
		}
	}
	if resp := get(t, srv.URL+"/api/v1/health", nil); resp.StatusCode != http.StatusOK {
		t.Fatalf("health = %d while help is disabled", resp.StatusCode)
	}
}
