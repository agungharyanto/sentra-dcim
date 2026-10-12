// Package config loads application configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config is the full application configuration.
type Config struct {
	Env             string
	LogLevel        string
	HTTPAddr        string
	ShutdownTimeout time.Duration
	DisplayTZ       *time.Location
	RateLimitRPS    float64
	RateLimitBurst  int
	TrustedProxies  []netip.Prefix
	DB              DB
	Redis           Redis
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
		Env:             p.str("APP_ENV", "production"),
		LogLevel:        strings.ToLower(p.str("APP_LOG_LEVEL", "info")),
		HTTPAddr:        p.str("APP_HTTP_ADDR", ":8080"),
		ShutdownTimeout: p.duration("APP_SHUTDOWN_TIMEOUT", 15*time.Second),
		RateLimitRPS:    p.float("APP_RATE_LIMIT_RPS", 20),
		RateLimitBurst:  p.int("APP_RATE_LIMIT_BURST", 40),
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

	cfg.TrustedProxies = p.prefixes("APP_TRUSTED_PROXIES", "127.0.0.1/32,::1/128,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16")

	tzName := p.str("APP_DISPLAY_TZ", "Asia/Makassar")
	tz, err := time.LoadLocation(tzName)
	if err != nil {
		p.errs = append(p.errs, fmt.Errorf("APP_DISPLAY_TZ: %w", err))
	}
	cfg.DisplayTZ = tz

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
