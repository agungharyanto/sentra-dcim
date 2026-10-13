package password

import (
	"strings"
	"testing"
)

var fast = Params{MemoryKiB: 8192, Time: 1, Threads: 1}

func TestHashVerify(t *testing.T) {
	h, err := Hash("correct horse battery", fast)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$m=8192,t=1,p=1$") {
		t.Fatalf("hash = %s", h)
	}
	if strings.Contains(h, "correct horse") {
		t.Fatal("hash contains the password")
	}
	ok, rehash, err := Verify("correct horse battery", h, fast)
	if err != nil || !ok || rehash {
		t.Fatalf("verify = %v %v %v", ok, rehash, err)
	}
	if ok, _, _ := Verify("wrong horse battery", h, fast); ok {
		t.Fatal("wrong password accepted")
	}
	h2, _ := Hash("correct horse battery", fast)
	if h == h2 {
		t.Fatal("salt not random")
	}
}

func TestNeedsRehashWhenParamsChange(t *testing.T) {
	h, _ := Hash("correct horse battery", fast)
	ok, rehash, err := Verify("correct horse battery", h, Params{MemoryKiB: 16384, Time: 1, Threads: 1})
	if err != nil || !ok || !rehash {
		t.Fatalf("verify = %v %v %v", ok, rehash, err)
	}
	if _, rehash, _ := Verify("nope nope nope", h, Params{MemoryKiB: 16384, Time: 1, Threads: 1}); rehash {
		t.Fatal("rehash flagged for a wrong password")
	}
}

func TestVerifyRejectsMalformed(t *testing.T) {
	for _, h := range []string{"", "plain", "$2a$10$abc", "$argon2i$v=19$m=8192,t=1,p=1$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5",
		"$argon2id$v=18$m=8192,t=1,p=1$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5", "$argon2id$v=19$m=8192,t=0,p=1$c2FsdHNhbHQ$a2V5a2V5a2V5a2V5a2V5"} {
		if _, _, err := Verify("x", h, fast); err == nil {
			t.Errorf("accepted malformed hash %q", h)
		}
	}
}

func TestPolicy(t *testing.T) {
	cases := map[string]bool{
		strings.Repeat("a", 11):   false,
		strings.Repeat("a", 12):   true,
		strings.Repeat("é", 12):   true, // characters, not bytes
		strings.Repeat("a", 128):  true,
		strings.Repeat("a", 129):  false,
		"pendek":                  false,
		"semua huruf kecil saja":  true, // no composition rules
		"\xff\xfe invalid utf8 x": false,
	}
	for pw, want := range cases {
		if got := CheckPolicy(pw) == nil; got != want {
			t.Errorf("CheckPolicy(%q) ok=%v, want %v", pw, got, want)
		}
	}
}
