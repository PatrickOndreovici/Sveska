package config

import "testing"

func lookup(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestLoadAppliesDefaults(t *testing.T) {
	cfg, err := Load(lookup(map[string]string{"DATABASE_URL": "postgres://example"}))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":8080")
	}
	if cfg.DatabaseURL != "postgres://example" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
}

func TestLoadUsesProvidedAddr(t *testing.T) {
	cfg, err := Load(lookup(map[string]string{"DATABASE_URL": "postgres://example", "HTTP_ADDR": ":9000"}))
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.HTTPAddr != ":9000" {
		t.Errorf("HTTPAddr = %q, want %q", cfg.HTTPAddr, ":9000")
	}
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	if _, err := Load(lookup(map[string]string{})); err == nil {
		t.Fatal("expected an error when DATABASE_URL is missing")
	}
}
