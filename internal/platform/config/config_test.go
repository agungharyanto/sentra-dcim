package config

import (
	"net/url"
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func base() map[string]string {
	return map[string]string{"DB_NAME": "sentra", "DB_USER": "sentra", "DB_PASSWORD": "p@ss/w:rd", "REDIS_PASSWORD": "r"}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := LoadFrom(env(base()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.HTTPAddr != ":8080" || cfg.InternalHTTPAddr != ":8081" || cfg.ShutdownTimeout != 15*time.Second || cfg.DB.Port != 5432 || cfg.DB.MaxConns != 10 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.DisplayTZ.String() != "Asia/Jakarta" {
		t.Fatalf("display tz = %s", cfg.DisplayTZ)
	}
	if cfg.Auth.SessionIdle != 30*time.Minute || cfg.Auth.SessionAbsolute != 12*time.Hour || cfg.Auth.LockThreshold != 5 ||
		cfg.Auth.LockDuration != 15*time.Minute || len(cfg.Auth.AllowedOrigins) != 1 {
		t.Fatalf("auth defaults: %+v", cfg.Auth)
	}
	if cfg.DefaultLocale != "id" {
		t.Fatalf("default locale = %s", cfg.DefaultLocale)
	}
	if len(cfg.TrustedProxies) != 5 {
		t.Fatalf("trusted proxies = %v", cfg.TrustedProxies)
	}
}

func TestDisplayTimezone(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want string
	}{
		{"default", nil, "Asia/Jakarta"},
		{"new name", map[string]string{"DISPLAY_TIMEZONE": "Asia/Makassar"}, "Asia/Makassar"},
		{"legacy name", map[string]string{"APP_DISPLAY_TZ": "Asia/Jayapura"}, "Asia/Jayapura"},
		{"new name wins", map[string]string{"DISPLAY_TIMEZONE": "UTC", "APP_DISPLAY_TZ": "Asia/Makassar"}, "UTC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := base()
			for k, v := range tc.env {
				m[k] = v
			}
			cfg, err := LoadFrom(env(m))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.DisplayTZ.String() != tc.want {
				t.Fatalf("display tz = %s, want %s", cfg.DisplayTZ, tc.want)
			}
		})
	}
}

func TestLoadRequired(t *testing.T) {
	_, err := LoadFrom(env(map[string]string{}))
	if err == nil {
		t.Fatal("expected error")
	}
	for _, k := range []string{"DB_NAME", "DB_USER", "DB_PASSWORD", "REDIS_PASSWORD"} {
		if !strings.Contains(err.Error(), k) {
			t.Errorf("error should mention %s: %v", k, err)
		}
	}
}

func TestLoadInvalidValues(t *testing.T) {
	cases := map[string]string{
		"DB_PORT":                "abc",
		"APP_SHUTDOWN_TIMEOUT":   "-1s",
		"APP_LOG_LEVEL":          "verbose",
		"DB_SSLMODE":             "maybe",
		"DISPLAY_TIMEZONE":       "Mars/Base",
		"APP_DISPLAY_TZ":         "Mars/Base",
		"APP_INTERNAL_HTTP_ADDR": ":8080",
		"DB_MAX_CONNS":           "5000",
		"APP_RATE_LIMIT_RPS":     "0",
		"APP_TRUSTED_PROXIES":    "not-a-cidr",
		"DEFAULT_LOCALE":         "fr",
		"APP_ALLOWED_ORIGINS":    "javascript:alert(1)",
		"AUTH_SESSION_IDLE":      "-5m",
		"AUTH_ARGON2_MEMORY_KIB": "16",
	}
	for k, v := range cases {
		t.Run(k, func(t *testing.T) {
			m := base()
			m[k] = v
			if _, err := LoadFrom(env(m)); err == nil || !strings.Contains(err.Error(), k) {
				t.Fatalf("expected error mentioning %s, got %v", k, err)
			}
		})
	}
}

func TestDSNEscapesCredentials(t *testing.T) {
	cfg, err := LoadFrom(env(base()))
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(cfg.DB.DSN())
	if err != nil {
		t.Fatalf("DSN not parseable: %v", err)
	}
	if pw, _ := u.User.Password(); pw != "p@ss/w:rd" {
		t.Fatalf("password round-trip failed: %q", pw)
	}
	if u.Query().Get("sslmode") != "disable" || u.Path != "/sentra" {
		t.Fatalf("unexpected DSN: %s", u.Redacted())
	}
}
