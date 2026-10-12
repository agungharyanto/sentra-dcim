package httpserver

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/agungharyanto/sentra-dcim/internal/platform/logging"
)

func TestRateLimiter(t *testing.T) {
	rl := NewRateLimiter(1, 3)
	h := NewRouter(logging.Discard(), rl, nil, func(r chi.Router) {
		r.Get("/x", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	})
	codes := map[int]int{}
	for range 5 {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
		req.RemoteAddr = "203.0.113.7:5555"
		h.ServeHTTP(rec, req)
		codes[rec.Code]++
		if rec.Code == http.StatusTooManyRequests && rec.Header().Get("Retry-After") == "" {
			t.Fatal("missing Retry-After")
		}
	}
	if codes[http.StatusNoContent] != 3 || codes[http.StatusTooManyRequests] != 2 {
		t.Fatalf("codes = %v", codes)
	}
	// A different client has its own bucket.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	req.RemoteAddr = "198.51.100.1:1"
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("other client got %d", rec.Code)
	}
}

func TestRealIP(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12")}
	var seen string
	h := realIP(trusted)(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { seen = r.RemoteAddr }))

	cases := []struct{ name, peer, xri, want string }{
		{"trusted proxy", "172.18.0.5:4000", "203.0.113.9", "203.0.113.9:0"},
		{"untrusted peer spoofing", "203.0.113.50:4000", "1.2.3.4", "203.0.113.50:4000"},
		{"trusted proxy garbage header", "172.18.0.5:4000", "not-an-ip", "172.18.0.5:4000"},
		{"no header", "172.18.0.5:4000", "", "172.18.0.5:4000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.peer
			if tc.xri != "" {
				req.Header.Set("X-Real-IP", tc.xri)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if seen != tc.want {
				t.Fatalf("RemoteAddr = %s, want %s", seen, tc.want)
			}
		})
	}
}

func TestRecovererReturnsJSON500(t *testing.T) {
	h := NewRouter(logging.Discard(), nil, nil, func(r chi.Router) {
		r.Get("/boom", func(http.ResponseWriter, *http.Request) { panic("boom") })
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/boom", nil))
	if rec.Code != http.StatusInternalServerError || rec.Header().Get("Content-Type") == "" {
		t.Fatalf("got %d", rec.Code)
	}
}
