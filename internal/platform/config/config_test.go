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
	if cfg.HTTPAddr != ":8080" || cfg.ShutdownTimeout != 15*time.Second || cfg.DB.Port != 5432 || cfg.DB.MaxConns != 10 {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if cfg.DisplayTZ.String() != "Asia/Makassar" {
		t.Fatalf("display tz = %s", cfg.DisplayTZ)
	}
	if len(cfg.TrustedProxies) != 5 {
		t.Fatalf("trusted proxies = %v", cfg.TrustedProxies)
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
		"DB_PORT":              "abc",
		"APP_SHUTDOWN_TIMEOUT": "-1s",
		"APP_LOG_LEVEL":        "verbose",
		"DB_SSLMODE":           "maybe",
		"APP_DISPLAY_TZ":       "Mars/Base",
		"DB_MAX_CONNS":         "5000",
		"APP_RATE_LIMIT_RPS":   "0",
		"APP_TRUSTED_PROXIES":  "not-a-cidr",
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
