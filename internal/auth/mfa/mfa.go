// Package mfa implements TOTP (RFC 6238: HMAC-SHA1, 6 digits, 30 s, ±1 step), AES-256-GCM encryption of the shared
// secrets, one-time recovery codes stored as HMAC-SHA256, and the QR matrix of the otpauth:// URI (D-062..D-064).
// Secrets, codes and recovery codes are never logged.
package mfa

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // RFC 6238 TOTP as implemented by Google/Microsoft Authenticator and Authy uses HMAC-SHA1
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"time"

	"rsc.io/qr"
)

// TOTP parameters (fixed: the only ones every authenticator app supports).
const (
	Digits     = 6
	Period     = 30 * time.Second
	Skew       = 1  // accepted steps before/after the current one
	SecretSize = 20 // bytes (160 bits, RFC 4226 recommendation)
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// Keys are the per-purpose keys derived from MFA_ENC_KEY.
type Keys struct {
	aead     cipher.AEAD
	recovery []byte
}

// NewKeys derives the encryption and recovery-code keys from the 32-byte master key.
func NewKeys(master []byte) (*Keys, error) {
	if len(master) != 32 {
		return nil, errors.New("mfa: master key must be 32 bytes")
	}
	block, err := aes.NewCipher(derive(master, "sentra/mfa/totp-secret/v1"))
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Keys{aead: aead, recovery: derive(master, "sentra/mfa/recovery-code/v1")}, nil
}

func derive(master []byte, label string) []byte {
	m := hmac.New(sha256.New, master)
	m.Write([]byte(label))
	return m.Sum(nil)
}

// NewSecret returns a random TOTP secret.
func NewSecret() ([]byte, error) {
	s := make([]byte, SecretSize)
	if _, err := rand.Read(s); err != nil {
		return nil, err
	}
	return s, nil
}

// Encrypt seals a secret; aad binds it to its owner (the user id), so a ciphertext cannot be moved to another account.
func (k *Keys) Encrypt(secret, aad []byte) ([]byte, error) {
	nonce := make([]byte, k.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return k.aead.Seal(nonce, nonce, secret, aad), nil
}

// Decrypt opens a sealed secret.
func (k *Keys) Decrypt(sealed, aad []byte) ([]byte, error) {
	n := k.aead.NonceSize()
	if len(sealed) < n+k.aead.Overhead() {
		return nil, errors.New("mfa: sealed secret too short")
	}
	out, err := k.aead.Open(nil, sealed[:n], sealed[n:], aad)
	if err != nil {
		return nil, errors.New("mfa: cannot decrypt secret (wrong MFA_ENC_KEY or tampered data)")
	}
	return out, nil
}

// Step returns the TOTP time step of t.
func Step(t time.Time) int64 { return t.Unix() / int64(Period/time.Second) }

// Code returns the TOTP code of secret at step (RFC 4226 dynamic truncation).
func Code(secret []byte, step int64) string {
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step)) //nolint:gosec // steps are positive
	m := hmac.New(sha1.New, secret)
	m.Write(msg[:])
	sum := m.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	v := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", Digits, v%1_000_000)
}

// Verify checks a code against the steps now-Skew..now+Skew that are newer than lastStep (anti-replay) and returns
// the matching step. Spaces are ignored; anything other than 6 digits never matches.
func Verify(secret []byte, code string, now time.Time, lastStep int64) (int64, bool) {
	code = strings.ReplaceAll(strings.TrimSpace(code), " ", "")
	if len(code) != Digits || strings.Trim(code, "0123456789") != "" {
		return 0, false
	}
	cur := Step(now)
	matched, ok := int64(0), false
	for s := cur - Skew; s <= cur+Skew; s++ {
		// Constant-time comparison and no early exit: timing does not reveal which step matched.
		if subtle.ConstantTimeCompare([]byte(Code(secret, s)), []byte(code)) == 1 && s > lastStep && !ok {
			matched, ok = s, true
		}
	}
	return matched, ok
}

// EncodeSecret is the base32 form typed into an authenticator app ("manual entry").
func EncodeSecret(secret []byte) string { return b32.EncodeToString(secret) }

// GroupedSecret formats the base32 secret in groups of four for reading and typing.
func GroupedSecret(secret []byte) string {
	s := EncodeSecret(secret)
	var b strings.Builder
	for i := 0; i < len(s); i += 4 {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(s[i:min(i+4, len(s))])
	}
	return b.String()
}

// URI is the otpauth:// key URI understood by Google Authenticator, Microsoft Authenticator and Authy.
func URI(issuer, account string, secret []byte) string {
	label := url.PathEscape(issuer) + ":" + url.PathEscape(account)
	q := url.Values{}
	q.Set("secret", EncodeSecret(secret))
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", "6")
	q.Set("period", "30")
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// QR returns the QR code of uri as rows of '1' (dark) and '0' (light) modules, without the quiet zone. The UI draws
// it as SVG (no image data URL, no CSP change).
func QR(uri string) ([]string, error) {
	c, err := qr.Encode(uri, qr.M)
	if err != nil {
		return nil, err
	}
	rows := make([]string, c.Size)
	for y := range c.Size {
		var b strings.Builder
		for x := range c.Size {
			if c.Black(x, y) {
				b.WriteByte('1')
			} else {
				b.WriteByte('0')
			}
		}
		rows[y] = b.String()
	}
	return rows, nil
}

// RecoveryCount is the number of recovery codes per account.
const RecoveryCount = 10

// recoveryAlphabet has no look-alikes (0/o, 1/l/i).
const recoveryAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

// NewRecoveryCodes returns RecoveryCount codes "xxxxx-xxxxx" (≈ 50 bits each).
func NewRecoveryCodes() ([]string, error) {
	out := make([]string, RecoveryCount)
	size := big.NewInt(int64(len(recoveryAlphabet)))
	for i := range out {
		var b strings.Builder
		for j := range 10 {
			if j == 5 {
				b.WriteByte('-')
			}
			n, err := rand.Int(rand.Reader, size)
			if err != nil {
				return nil, err
			}
			b.WriteByte(recoveryAlphabet[n.Int64()])
		}
		out[i] = b.String()
	}
	return out, nil
}

// NormalizeRecovery lowercases and removes spaces/hyphens, so "ABCDE FGHJK" matches "abcde-fghjk".
func NormalizeRecovery(code string) string {
	return strings.NewReplacer(" ", "", "-", "").Replace(strings.ToLower(strings.TrimSpace(code)))
}

// LooksLikeRecovery reports whether input has the shape of a recovery code (10 characters of the alphabet).
func LooksLikeRecovery(code string) bool {
	n := NormalizeRecovery(code)
	return len(n) == 10 && strings.Trim(n, recoveryAlphabet) == ""
}

// HashRecovery is the stored form of a recovery code (HMAC-SHA256 with a key derived from MFA_ENC_KEY, so a
// database dump alone does not allow offline guessing).
func (k *Keys) HashRecovery(code string) []byte {
	m := hmac.New(sha256.New, k.recovery)
	m.Write([]byte(NormalizeRecovery(code)))
	return m.Sum(nil)
}
