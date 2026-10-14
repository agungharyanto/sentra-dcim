package handler

import "testing"

func TestSafeCell(t *testing.T) {
	for in, want := range map[string]string{
		"":                         "",
		"plain":                    "plain",
		"=1+1":                     "'=1+1",
		"+cmd":                     "'+cmd",
		"-2":                       "'-2",
		"@SUM(A1)":                 "'@SUM(A1)",
		"\t=1":                     "'\t=1",
		"\r=1":                     "'\r=1",
		"a=1":                      "a=1",
		`{"note": "=x"}`:           `{"note": "=x"}`,
		"2026-10-14T10:00:00Z":     "2026-10-14T10:00:00Z",
		"'already quoted":          "'already quoted",
		"=HYPERLINK(\"http://x\")": "'=HYPERLINK(\"http://x\")",
	} {
		if got := SafeCell(in); got != want {
			t.Errorf("SafeCell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLikeEscape(t *testing.T) {
	if got := likeEscape(`a%b_c\d`); got != `a\%b\_c\\d` {
		t.Fatalf("likeEscape = %q", got)
	}
}
