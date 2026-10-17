package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIfMatch(t *testing.T) {
	cases := map[string]struct {
		code int
		v    int
	}{
		"":       {http.StatusPreconditionRequired, 0},
		"*":      {http.StatusPreconditionRequired, 0},
		`"3"`:    {0, 3},
		`W/"12"`: {0, 12},
		"3":      {http.StatusPreconditionFailed, 0},
		`"0"`:    {http.StatusPreconditionFailed, 0},
		`"x"`:    {http.StatusPreconditionFailed, 0},
	}
	for hdr, want := range cases {
		r := httptest.NewRequest(http.MethodPatch, "/", nil)
		if hdr != "" {
			r.Header.Set("If-Match", hdr)
		}
		w := httptest.NewRecorder()
		v, ok := ifMatch(w, r)
		if want.code == 0 {
			if !ok || v != want.v {
				t.Errorf("%q → %d %t", hdr, v, ok)
			}
		} else if ok || w.Code != want.code {
			t.Errorf("%q → ok=%t code %d, want %d", hdr, ok, w.Code, want.code)
		}
	}
	if ETag(7) != `"7"` {
		t.Error("ETag format")
	}
}
