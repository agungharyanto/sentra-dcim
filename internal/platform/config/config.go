// Package config loads application configuration from environment variables.
package config

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

// DefaultDisplayTimezone is the display timezone when DISPLAY_TIMEZONE is not set (WIB, UTC+7).
const DefaultDisplayTimezone = "Asia/Jakarta"

// DefaultLocale is the UI locale when DEFAULT_LOCALE is not set.
const DefaultLocale = "id"

// SupportedLocales lists the UI locales with a complete translation catalog (web/src/i18n/locales).
var SupportedLocales = []string{"id", "en"}

// Config is the full application configuration.
type Config struct {
	Env      string
	LogLevel string
	// HTTPAddr serves the public-facing API (proxied by Caddy's public site); responses are minimal.
	HTTPAddr string
	// InternalHTTPAddr serves the same API with operational details (e.g. full /version). Reachable only via the
	// Caddy listener bound to host 127.0.0.1; never proxy it publicly.
	InternalHTTPAddr string
	ShutdownTimeout  time.Duration
	DisplayTZ        *time.Location
	// DefaultLocale is the server-wide UI locale (one of SupportedLocales); users may switch in the UI.
	DefaultLocale  string
	RateLimitRPS   float64
	RateLimitBurst int
	TrustedProxies []netip.Prefix
	// AuditRetentionDays: audit events older than this are archived and pruned by `make audit-prune` (min 90).
	AuditRetentionDays int
	DB                 DB
	Redis              Redis
	Auth               Auth
}

// Auth holds authentication settings (ADR-003, v0.3.1).
type Auth struct {
	// argon2id parameters for new hashes; existing hashes keep their own parameters (rehashed on login).
	Argon2MemoryKiB uint32
	Argon2Time      uint32
	Argon2Threads   uint8
	SessionIdle     time.Duration // sliding idle timeout (30 min)
	SessionAbsolute time.Duration // absolute session lifetime (12 h)
	LockThreshold   int           // consecutive wrong passwords before the account is locked (5)
	LockDuration    time.Duration // lock length (15 min)
	RateIPLimit     int           // login attempts per client IP per RateWindow
	RateUserLimit   int           // login attempts per username per RateWindow
	RateWindow      time.Duration
	// AllowedOrigins are the exact origins (scheme://host[:port]) accepted for unsafe requests (CSRF).
	AllowedOrigins []string
	// MFAKey (MFA_ENC_KEY, 32 bytes base64) encrypts TOTP secrets (AES-256-GCM) and keys the recovery-code HMAC
	// (D-063, D-064). Never logged.
	MFAKey []byte `json:"-"`
	// MFAIssuer is the account label prefix shown by authenticator apps.
	MFAIssuer string
}

// DB holds PostgreSQL connection settings.
type DB struct {
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	SSLMode  string
	MaxConns int32
}

// Redis holds Redis connection settings.
type Redis struct {
	Addr     string
	Password string
	DB       int
}

// DSN returns a PostgreSQL connection URL with credentials safely escaped.
func (d DB) DSN() string {
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(d.User, d.Password),
		Host:     net.JoinHostPort(d.Host, strconv.Itoa(d.Port)),
		Path:     "/" + d.Name,
		RawQuery: url.Values{"sslmode": {d.SSLMode}}.Encode(),
	}
	return u.String()
}

// Load reads configuration from the process environment.
func Load() (Config, error) { return LoadFrom(os.LookupEnv) }

// LoadFrom reads configuration using the given lookup function (testable).
func LoadFrom(lookup func(string) (string, bool)) (Config, error) {
	p := parser{lookup: lookup}
	cfg := Config{
		Env:              p.str("APP_ENV", "production"),
		LogLevel:         strings.ToLower(p.str("APP_LOG_LEVEL", "info")),
		HTTPAddr:         p.str("APP_HTTP_ADDR", ":8080"),
		InternalHTTPAddr: p.str("APP_INTERNAL_HTTP_ADDR", ":8081"),
		ShutdownTimeout:  p.duration("APP_SHUTDOWN_TIMEOUT", 15*time.Second),
		DefaultLocale:    strings.ToLower(p.str("DEFAULT_LOCALE", DefaultLocale)),
		RateLimitRPS:     p.float("APP_RATE_LIMIT_RPS", 20),
		RateLimitBurst:   p.int("APP_RATE_LIMIT_BURST", 40),
		DB: DB{
			Host:     p.str("DB_HOST", "127.0.0.1"),
			Port:     p.int("DB_PORT", 5432),
			Name:     p.required("DB_NAME"),
			User:     p.required("DB_USER"),
			Password: p.required("DB_PASSWORD"),
			SSLMode:  p.str("DB_SSLMODE", "disable"),
			MaxConns: p.int32("DB_MAX_CONNS", 10, 1, 1000),
		},
		Redis: Redis{
			Addr:     p.str("REDIS_ADDR", "127.0.0.1:6379"),
			Password: p.required("REDIS_PASSWORD"),
			DB:       p.int("REDIS_DB", 0),
		},
	}

	cfg.Auth = Auth{
		Argon2MemoryKiB: uint32(p.int32("AUTH_ARGON2_MEMORY_KIB", 65536, 8192, 1048576)), //nolint:gosec // bounded
		Argon2Time:      uint32(p.int32("AUTH_ARGON2_TIME", 2, 1, 10)),                   //nolint:gosec // bounded
		Argon2Threads:   uint8(p.int32("AUTH_ARGON2_THREADS", 2, 1, 16)),                 //nolint:gosec // bounded
		SessionIdle:     p.duration("AUTH_SESSION_IDLE", 30*time.Minute),
		SessionAbsolute: p.duration("AUTH_SESSION_ABSOLUTE", 12*time.Hour),
		LockThreshold:   int(p.int32("AUTH_LOCK_THRESHOLD", 5, 1, 100)),
		LockDuration:    p.duration("AUTH_LOCK_DURATION", 15*time.Minute),
		RateIPLimit:     int(p.int32("AUTH_RATE_IP_LIMIT", 30, 1, 100000)),
		RateUserLimit:   int(p.int32("AUTH_RATE_USER_LIMIT", 10, 1, 100000)),
		RateWindow:      p.duration("AUTH_RATE_WINDOW", 10*time.Minute),
		AllowedOrigins:  p.origins("APP_ALLOWED_ORIGINS", "https://localhost"),
		MFAKey:          p.key32("MFA_ENC_KEY"),
		MFAIssuer:       p.str("MFA_ISSUER", "Sentra DCIM"),
	}
	if cfg.Auth.SessionIdle > cfg.Auth.SessionAbsolute {
		p.errs = append(p.errs, errors.New("AUTH_SESSION_IDLE must not exceed AUTH_SESSION_ABSOLUTE"))
	}

	cfg.AuditRetentionDays = int(p.int32("AUDIT_RETENTION_DAYS", 365, 90, 36500))

	cfg.TrustedProxies = p.prefixes("APP_TRUSTED_PROXIES", "127.0.0.1/32,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")

	// DISPLAY_TIMEZONE replaces APP_DISPLAY_TZ (v0.1.8); the old name is still honoured for existing .env files.
	tzKey := "DISPLAY_TIMEZONE"
	if p.str(tzKey, "") == "" && p.str("APP_DISPLAY_TZ", "") != "" {
		tzKey = "APP_DISPLAY_TZ"
	}
	tz, err := time.LoadLocation(p.str(tzKey, DefaultDisplayTimezone))
	if err != nil {
		p.errs = append(p.errs, fmt.Errorf("%s: %w", tzKey, err))
	}
	cfg.DisplayTZ = tz

	if cfg.HTTPAddr == cfg.InternalHTTPAddr {
		p.errs = append(p.errs, errors.New("APP_HTTP_ADDR and APP_INTERNAL_HTTP_ADDR must differ"))
	}

	if !slices.Contains(SupportedLocales, cfg.DefaultLocale) {
		p.errs = append(p.errs, fmt.Errorf("DEFAULT_LOCALE: unsupported locale %q (want one of %s)",
			cfg.DefaultLocale, strings.Join(SupportedLocales, ", ")))
	}

	switch cfg.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		p.errs = append(p.errs, fmt.Errorf("APP_LOG_LEVEL: invalid value %q", cfg.LogLevel))
	}
	switch cfg.DB.SSLMode {
	case "disable", "require", "verify-ca", "verify-full":
	default:
		p.errs = append(p.errs, fmt.Errorf("DB_SSLMODE: invalid value %q", cfg.DB.SSLMode))
	}
	if cfg.RateLimitRPS <= 0 || cfg.RateLimitBurst <= 0 {
		p.errs = append(p.errs, errors.New("APP_RATE_LIMIT_RPS and APP_RATE_LIMIT_BURST must be positive"))
	}

	return cfg, errors.Join(p.errs...)
}

type parser struct {
	lookup func(string) (string, bool)
	errs   []error
}

func (p *parser) str(key, def string) string {
	if v, ok := p.lookup(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return def
}

// key32 reads a required 32-byte key encoded as standard or URL-safe base64 (e.g. `openssl rand -base64 32`). The
// value is never included in error messages.
func (p *parser) key32(key string) []byte {
	v := strings.TrimSpace(p.required(key))
	if v == "" {
		return nil
	}
	for _, enc := range []*base64.Encoding{base64.StdEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.RawURLEncoding} {
		if b, err := enc.DecodeString(v); err == nil && len(b) == 32 {
			return b
		}
	}
	p.errs = append(p.errs, fmt.Errorf("%s: must be 32 random bytes in base64 (openssl rand -base64 32)", key))
	return nil
}

func (p *parser) required(key string) string {
	v, ok := p.lookup(key)
	if !ok || strings.TrimSpace(v) == "" {
		p.errs = append(p.errs, fmt.Errorf("%s: required", key))
		return ""
	}
	return v
}

func (p *parser) int(key string, def int) int {
	v := p.str(key, "")
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		p.errs = append(p.errs, fmt.Errorf("%s: not an integer: %q", key, v))
		return def
	}
	return n
}

func (p *parser) int32(key string, def, lo, hi int32) int32 {
	n := p.int(key, int(def))
	if n < int(lo) || n > int(hi) {
		p.errs = append(p.errs, fmt.Errorf("%s: must be between %d and %d, got %d", key, lo, hi, n))
		return def
	}
	return int32(n) //nolint:gosec // bounds checked above
}

func (p *parser) prefixes(key, def string) []netip.Prefix {
	var out []netip.Prefix
	for _, s := range strings.Split(p.str(key, def), ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		pfx, err := netip.ParsePrefix(s)
		if err != nil {
			p.errs = append(p.errs, fmt.Errorf("%s: invalid CIDR %q", key, s))
			continue
		}
		out = append(out, pfx.Masked())
	}
	return out
}

// origins parses a comma-separated list of exact origins (scheme://host[:port], no path).
func (p *parser) origins(key, def string) []string {
	var out []string
	for _, s := range strings.Split(p.str(key, def), ",") {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		u, err := url.Parse(s)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || (u.Path != "" && u.Path != "/") ||
			u.RawQuery != "" || u.User != nil {
			p.errs = append(p.errs, fmt.Errorf("%s: invalid origin %q (want scheme://host[:port])", key, s))
			continue
		}
		out = append(out, u.Scheme+"://"+strings.ToLower(u.Host))
	}
	if len(out) == 0 {
		p.errs = append(p.errs, fmt.Errorf("%s: at least one origin required", key))
	}
	return out
}

func (p *parser) float(key string, def float64) float64 {
	v := p.str(key, "")
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		p.errs = append(p.errs, fmt.Errorf("%s: not a number: %q", key, v))
		return def
	}
	return f
}

func (p *parser) duration(key string, def time.Duration) time.Duration {
	v := p.str(key, "")
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		p.errs = append(p.errs, fmt.Errorf("%s: invalid duration: %q", key, v))
		return def
	}
	return d
}
