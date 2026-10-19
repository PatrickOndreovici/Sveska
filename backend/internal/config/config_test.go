package config

import (
	"testing"
	"time"
)

func lookup(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func required() map[string]string {
	return map[string]string{
		"DATABASE_URL": "postgres://example",
		"APP_ORIGIN":   "http://localhost:5180",
	}
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load(lookup(required()))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}
	if !cfg.CookieSecure {
		t.Error("CookieSecure should default to true")
	}
	if cfg.SessionTTL != 30*24*time.Hour {
		t.Errorf("SessionTTL = %v", cfg.SessionTTL)
	}
}

func TestLoadReadsOptionalValues(t *testing.T) {
	env := required()
	env["HTTP_ADDR"] = ":9000"
	env["COOKIE_SECURE"] = "false"
	cfg, err := Load(lookup(env))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPAddr != ":9000" {
		t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":9000")
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure should be false")
	}
}

func TestLoadRejectsBadInput(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"missing database url", map[string]string{"APP_ORIGIN": "http://localhost:5180"}},
		{"missing app origin", map[string]string{"DATABASE_URL": "postgres://example"}},
		{"origin with path", map[string]string{"DATABASE_URL": "postgres://example", "APP_ORIGIN": "http://localhost:5180/app"}},
		{"origin without scheme", map[string]string{"DATABASE_URL": "postgres://example", "APP_ORIGIN": "localhost:5180"}},
		{"bad cookie flag", map[string]string{"DATABASE_URL": "postgres://example", "APP_ORIGIN": "http://localhost:5180", "COOKIE_SECURE": "maybe"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Load(lookup(tt.env)); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
