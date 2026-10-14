package mfa

import (
	"bytes"
	"net/url"
	"strings"
	"testing"
	"time"
)

// RFC 6238 Appendix B (SHA1), truncated to 6 digits.
func TestRFC6238Vectors(t *testing.T) {
	secret := []byte("12345678901234567890")
	for _, v := range []struct {
		unix int64
		want string
	}{{59, "287082"}, {1111111109, "081804"}, {1111111111, "050471"}, {1234567890, "005924"}, {2000000000, "279037"}, {20000000000, "353130"}} {
		if got := Code(secret, Step(time.Unix(v.unix, 0))); got != v.want {
			t.Errorf("T=%d: got %s want %s", v.unix, got, v.want)
		}
	}
}

func TestVerifyWindowAndReplay(t *testing.T) {
	secret := []byte("12345678901234567890")
	now := time.Unix(1111111111, 0)
	cur := Step(now)
	for _, d := range []int64{-1, 0, 1} {
		if s, ok := Verify(secret, Code(secret, cur+d), now, 0); !ok || s != cur+d {
			t.Errorf("step %+d not accepted", d)
		}
	}
	for _, d := range []int64{-2, 2} {
		if _, ok := Verify(secret, Code(secret, cur+d), now, 0); ok {
			t.Errorf("step %+d accepted", d)
		}
	}
	// Anti-replay: a step at or before the last used one is refused, even inside the window.
	if _, ok := Verify(secret, Code(secret, cur), now, cur); ok {
		t.Error("replayed code accepted")
	}
	if _, ok := Verify(secret, Code(secret, cur-1), now, cur-1); ok {
		t.Error("replayed previous-step code accepted")
	}
	if _, ok := Verify(secret, Code(secret, cur+1), now, cur); !ok {
		t.Error("next step refused after current was used")
	}
	for _, bad := range []string{"", "12345", "1234567", "12a456", "      "} {
		if _, ok := Verify(secret, bad, now, 0); ok {
			t.Errorf("malformed %q accepted", bad)
		}
	}
	if _, ok := Verify(secret, " "+Code(secret, cur)[:3]+" "+Code(secret, cur)[3:], now, 0); !ok {
		t.Error("code with spaces refused")
	}
}

func TestEncryptBindsOwner(t *testing.T) {
	k, err := NewKeys(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := NewSecret()
	sealed, err := k.Encrypt(secret, []byte("user-a"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(sealed, secret) {
		t.Fatal("ciphertext contains the secret")
	}
	if got, err := k.Decrypt(sealed, []byte("user-a")); err != nil || !bytes.Equal(got, secret) {
		t.Fatalf("decrypt: %v", err)
	}
	if _, err := k.Decrypt(sealed, []byte("user-b")); err == nil {
		t.Fatal("secret opened for another user")
	}
	other, _ := NewKeys(bytes.Repeat([]byte{8}, 32))
	if _, err := other.Decrypt(sealed, []byte("user-a")); err == nil {
		t.Fatal("secret opened with another key")
	}
	if _, err := NewKeys([]byte("short")); err == nil {
		t.Fatal("short master key accepted")
	}
}

func TestRecoveryCodes(t *testing.T) {
	k, _ := NewKeys(bytes.Repeat([]byte{1}, 32))
	codes, err := NewRecoveryCodes()
	if err != nil || len(codes) != RecoveryCount {
		t.Fatalf("codes: %v %d", err, len(codes))
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if len(c) != 11 || c[5] != '-' || !LooksLikeRecovery(c) || seen[c] {
			t.Fatalf("bad or duplicate code %q", c)
		}
		seen[c] = true
	}
	c := codes[0]
	if !bytes.Equal(k.HashRecovery(c), k.HashRecovery(strings.ToUpper(strings.ReplaceAll(c, "-", " ")))) {
		t.Fatal("normalisation differs")
	}
	other, _ := NewKeys(bytes.Repeat([]byte{2}, 32))
	if bytes.Equal(k.HashRecovery(c), other.HashRecovery(c)) {
		t.Fatal("recovery hash does not depend on the key")
	}
	if LooksLikeRecovery("123456") {
		t.Fatal("TOTP code taken for a recovery code")
	}
}

func TestURIAndQR(t *testing.T) {
	secret := []byte("12345678901234567890")
	u, err := url.Parse(URI("Sentra DCIM", "admin.satu", secret))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if u.Scheme != "otpauth" || u.Host != "totp" || u.Path != "/Sentra DCIM:admin.satu" || q.Get("secret") != "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ" ||
		q.Get("issuer") != "Sentra DCIM" || q.Get("digits") != "6" || q.Get("period") != "30" || q.Get("algorithm") != "SHA1" {
		t.Fatalf("uri = %s", u)
	}
	if g := GroupedSecret(secret); g != "GEZD GNBV GY3T QOJQ GEZD GNBV GY3T QOJQ" {
		t.Fatalf("grouped = %q", g)
	}
	rows, err := QR(u.String())
	if err != nil || len(rows) < 21 || len(rows[0]) != len(rows) {
		t.Fatalf("qr %d rows: %v", len(rows), err)
	}
}
