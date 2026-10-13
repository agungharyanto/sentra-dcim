package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/agungharyanto/sentra-dcim/internal/api"
	authhandler "github.com/agungharyanto/sentra-dcim/internal/auth/handler"
	helphandler "github.com/agungharyanto/sentra-dcim/internal/help/handler"
	"github.com/agungharyanto/sentra-dcim/internal/platform/httpserver"
	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
	systemhandler "github.com/agungharyanto/sentra-dcim/internal/system/handler"
	systemservice "github.com/agungharyanto/sentra-dcim/internal/system/service"
	uihandler "github.com/agungharyanto/sentra-dcim/internal/ui/handler"
	uiservice "github.com/agungharyanto/sentra-dcim/internal/ui/service"
)

func router(t *testing.T, internal bool) http.Handler {
	t.Helper()
	sys := systemservice.New(logging.Discard(), time.UTC)
	m := api.Modules{
		System: systemhandler.New(sys),
		UI:     uihandler.New(uiservice.New("id", []string{"id", "en"}, time.UTC)),
		Help:   helphandler.New(nil, "id", []string{"id", "en"}),
		// No service: requests without a cookie are rejected before it is used; login is never called here.
		Auth: authhandler.New(nil, logging.Discard(), []string{"https://localhost"}),
	}
	if internal {
		m.System = systemhandler.NewInternal(sys)
	}
	return httpserver.NewRouter(logging.Discard(), nil, nil, api.Mount(m, internal))
}

// routes enumerates every registered "METHOD /path" of the router.
func routes(t *testing.T, h http.Handler) []string {
	t.Helper()
	var out []string
	err := chi.Walk(h.(chi.Routes), func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		out = append(out, method+" "+strings.ReplaceAll(route, "/*/", "/"))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// concrete turns a route pattern into a requestable path.
func concrete(route string) string {
	r := strings.NewReplacer("{id}", "00000000-0000-4000-8000-000000000000", "{slug}", "dashboard", "*", "x")
	return r.Replace(route)
}

// TestDefaultDeny requests every registered route without a session: everything outside the allowlist must answer
// 401 "unauthenticated". A new route registered outside the protected group fails this test.
func TestDefaultDeny(t *testing.T) {
	for _, tc := range []struct {
		name     string
		internal bool
		allow    []string
	}{
		{"public listener", false, api.PublicRoutes},
		{"internal listener", true, api.InternalPublicRoutes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := router(t, tc.internal)
			all := routes(t, h)
			if len(all) < 10 {
				t.Fatalf("suspiciously few routes: %v", all)
			}
			for _, a := range tc.allow {
				if !slices.Contains(all, a) {
					t.Errorf("allowlisted route %q is not registered", a)
				}
			}
			for _, rt := range all {
				if slices.Contains(tc.allow, rt) {
					continue
				}
				method, path, _ := strings.Cut(rt, " ")
				req := httptest.NewRequest(method, concrete(path), strings.NewReader("{}"))
				req.Header.Set("Origin", "https://localhost")
				req.Header.Set(authhandler.CSRFHeader, "x")
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
				var body httpserver.ErrorBody
				_ = json.NewDecoder(rec.Body).Decode(&body)
				if rec.Code != http.StatusUnauthorized || body.Error.Code != authhandler.CodeUnauthenticated {
					t.Errorf("%s without session: got %d %q, want 401 unauthenticated", rt, rec.Code, body.Error.Code)
				}
			}
		})
	}
}

// TestPublicRoutesReachable checks the allowlist really is reachable without a session.
func TestPublicRoutesReachable(t *testing.T) {
	h := router(t, true)
	for _, p := range []string{"/api/v1/health", "/api/v1/version"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET %s = %d, want 200", p, rec.Code)
		}
	}
	// Login is public but still CSRF-protected: no Origin/header → 403 before any credential check.
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader("{}")))
	if rec.Code != http.StatusForbidden {
		t.Errorf("POST /auth/login without CSRF = %d, want 403", rec.Code)
	}
}
