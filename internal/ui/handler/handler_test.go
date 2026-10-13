package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/agungharyanto/sentra-dcim/internal/platform/buildinfo"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	"github.com/agungharyanto/sentra-dcim/internal/ui/handler"
	"github.com/agungharyanto/sentra-dcim/internal/ui/service"
)

func TestBootstrap(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Makassar")
	if err != nil {
		t.Fatal(err)
	}
	svc := service.New("en", []string{"id", "en"}, loc)
	r := httpserver.NewRouter(logging.Discard(), nil, nil, func(r chi.Router) { handler.New(svc).Routes(r) })
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	resp, err := http.Get(srv.URL + "/api/v1/ui/bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Fatalf("Cache-Control = %q", cc)
	}

	// Decode generically to pin the JSON contract (field names), not just the Go struct.
	var doc map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"app_name", "version", "default_locale", "supported_locales", "timezone", "menus"} {
		if _, ok := doc[k]; !ok {
			t.Fatalf("missing field %q in %v", k, doc)
		}
	}
	if len(doc) != 6 {
		t.Fatalf("unexpected extra fields: %v", doc)
	}
	if doc["version"] != buildinfo.Version || doc["default_locale"] != "en" || doc["timezone"] != "Asia/Makassar" {
		t.Fatalf("unexpected values: %v", doc)
	}

	menus := doc["menus"].([]any)
	if len(menus) == 0 {
		t.Fatal("no menus")
	}
	allowed := []string{service.StatusAvailable, service.StatusComingSoon, service.StatusNotConnected}
	prev := -1.0
	keys := map[string]bool{}
	for _, m := range menus {
		item := m.(map[string]any)
		if len(item) != 5 {
			t.Fatalf("menu fields = %v (labels must not be sent)", item)
		}
		key, _ := item["key"].(string)
		if key == "" || keys[key] {
			t.Fatalf("empty or duplicate key in %v", item)
		}
		keys[key] = true
		if !slices.Contains(allowed, item["status"].(string)) {
			t.Fatalf("invalid status in %v", item)
		}
		if o := item["order"].(float64); o <= prev {
			t.Fatalf("menus not sorted by order: %v", menus)
		} else {
			prev = o
		}
	}
	first := menus[0].(map[string]any)
	if first["key"] != "dashboard" || first["status"] != service.StatusAvailable || first["path"] != "/" {
		t.Fatalf("first menu = %v", first)
	}
}
