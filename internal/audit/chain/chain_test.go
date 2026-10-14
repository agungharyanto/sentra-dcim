package chain

import (
	"encoding/hex"
	"testing"
)

func TestCanonicalIsLengthPrefixed(t *testing.T) {
	var a, b [14]string
	a[6], a[7] = "ab", "c" // "2:ab1:c" vs "1:a2:bc": moving a boundary must change the form
	b[6], b[7] = "a", "bc"
	if string(Canonical(a)) == string(Canonical(b)) {
		t.Fatal("field boundaries are ambiguous")
	}
	var u [14]string
	u[5] = "ü" // octet length, not rune count (same as SQL octet_length)
	if got := string(Canonical(u)); got != "0:0:0:0:0:2:ü0:0:0:0:0:0:0:0:" {
		t.Fatalf("canonical = %q", got)
	}
}

func TestHashChainsPrev(t *testing.T) {
	c := Canonical([14]string{"1"})
	if hex.EncodeToString(Hash(zero, c)) == hex.EncodeToString(Hash(Hash(zero, c), c)) {
		t.Fatal("hash ignores prev")
	}
}

func TestParseAnchors(t *testing.T) {
	h := "aa" + "00"
	for len(h) < 64 {
		h += "00"
	}
	got, err := ParseAnchors("\n{}\n{\"seq\":3,\"hash\":\"" + h + "\",\"at\":\"2026-10-14T00:00:00Z\"}\n  \n")
	if err != nil || len(got) != 1 || got[0].Seq != 3 || got[0].Hash != h {
		t.Fatalf("anchors = %+v, %v", got, err)
	}
	if _, err := ParseAnchors("not json"); err == nil {
		t.Fatal("garbage accepted")
	}
}
