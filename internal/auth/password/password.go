// Package password hashes and verifies passwords with argon2id (PHC string format) and checks the length policy
// (ADR-003: 12–128 characters, no composition rules).
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// Length policy (characters, not bytes).
const (
	MinLength = 12
	MaxLength = 128
)

// ErrPolicy is returned when a new password violates the length policy.
var ErrPolicy = fmt.Errorf("password must be %d to %d characters", MinLength, MaxLength)

// Params are argon2id parameters.
type Params struct {
	MemoryKiB uint32
	Time      uint32
	Threads   uint8
}

const (
	saltLen = 16
	keyLen  = 32
)

var b64 = base64.RawStdEncoding

// CheckPolicy validates the length of a new password (Unicode characters).
func CheckPolicy(pw string) error {
	if !utf8.ValidString(pw) {
		return ErrPolicy
	}
	if n := utf8.RuneCountInString(pw); n < MinLength || n > MaxLength {
		return ErrPolicy
	}
	return nil
}

// Hash returns an argon2id PHC string: $argon2id$v=19$m=<KiB>,t=<time>,p=<threads>$<salt>$<hash>.
func Hash(pw string, p Params) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(pw), salt, p.Time, p.MemoryKiB, p.Threads, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.MemoryKiB, p.Time, p.Threads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// Verify checks pw against an encoded hash in constant time. needsRehash is true when the hash was made with
// parameters different from current (the caller may store a fresh hash after a successful login).
func Verify(pw, encoded string, current Params) (ok, needsRehash bool, err error) {
	p, salt, key, err := decode(encoded)
	if err != nil {
		return false, false, err
	}
	got := argon2.IDKey([]byte(pw), salt, p.Time, p.MemoryKiB, p.Threads, uint32(len(key))) //nolint:gosec // len(key) is small
	ok = subtle.ConstantTimeCompare(got, key) == 1
	return ok, ok && p != current, nil
}

func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return Params{}, nil, nil, errors.New("not an argon2id hash")
	}
	var v int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &v); err != nil || v != argon2.Version {
		return Params{}, nil, nil, errors.New("unsupported argon2 version")
	}
	var p Params
	var threads uint32
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.MemoryKiB, &p.Time, &threads); err != nil {
		return Params{}, nil, nil, errors.New("invalid argon2 parameters")
	}
	if threads == 0 || threads > 255 || p.Time == 0 || p.MemoryKiB < 8 {
		return Params{}, nil, nil, errors.New("invalid argon2 parameters")
	}
	p.Threads = uint8(threads)
	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) < 8 {
		return Params{}, nil, nil, errors.New("invalid salt")
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) < 16 {
		return Params{}, nil, nil, errors.New("invalid key")
	}
	return p, salt, key, nil
}
