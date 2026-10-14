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
	for _, k := range []string{"app_name", "version", "default_locale", "supported_locales", "timezone", "menu_groups", "menus"} {
		if _, ok := doc[k]; !ok {
			t.Fatalf("missing field %q in %v", k, doc)
		}
	}
	if len(doc) != 7 {
		t.Fatalf("unexpected extra fields: %v", doc)
	}
	if doc["version"] != buildinfo.Version || doc["default_locale"] != "en" || doc["timezone"] != "Asia/Makassar" {
		t.Fatalf("unexpected values: %v", doc)
	}

	groupRank := map[string]int{}
	prevOrder := -1.0
	for i, g := range doc["menu_groups"].([]any) {
		grp := g.(map[string]any)
		if len(grp) != 2 || grp["key"] == "" {
			t.Fatalf("menu group fields = %v (labels must not be sent)", grp)
		}
		if o := grp["order"].(float64); o <= prevOrder {
			t.Fatalf("menu_groups not sorted by order: %v", doc["menu_groups"])
		} else {
			prevOrder = o
		}
		groupRank[grp["key"].(string)] = i
	}

	menus := doc["menus"].([]any)
	if len(menus) == 0 {
		t.Fatal("no menus")
	}
	allowed := []string{service.StatusAvailable, service.StatusComingSoon, service.StatusNotConnected}
	prevGroup, prev := -1, -1.0
	keys := map[string]bool{}
	for _, m := range menus {
		item := m.(map[string]any)
		if len(item) != 7 {
			t.Fatalf("menu fields = %v (labels must not be sent)", item)
		}
		rank, ok := groupRank[item["group"].(string)]
		if !ok {
			t.Fatalf("menu %v: group not in menu_groups", item)
		}
		if rank != prevGroup {
			if rank < prevGroup {
				t.Fatalf("menus not sorted by group order: %v", menus)
			}
			prevGroup, prev = rank, -1
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
			t.Fatalf("menus not sorted by order within group: %v", menus)
		} else {
			prev = o
		}
	}
	first := menus[0].(map[string]any)
	if first["key"] != "dashboard" || first["status"] != service.StatusAvailable || first["path"] != "/dashboard" ||
		first["group"] != service.GroupMonitoring {
		t.Fatalf("first menu = %v", first)
	}
}
