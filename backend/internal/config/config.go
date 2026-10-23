// Package config reads process configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// Config holds settings the API server needs at startup.
type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	ShutdownTimeout time.Duration

	// AppOrigin is the only origin allowed to send state-changing requests
	// (scheme://host[:port], no path). Used for CSRF protection.
	AppOrigin string
	// CookieSecure marks the session cookie Secure (HTTPS only). Defaults to
	// true; local HTTP development sets COOKIE_SECURE=false.
	CookieSecure bool
	SessionTTL   time.Duration

	// FileStorageDir is where uploaded PDFs are kept, created if missing.
	FileStorageDir string
}

// Load builds a Config from the given lookup function (normally os.Getenv).
// Passing the lookup in keeps Load deterministic in tests.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		HTTPAddr:        getenv("HTTP_ADDR"),
		DatabaseURL:     getenv("DATABASE_URL"),
		ShutdownTimeout: 10 * time.Second,
		AppOrigin:       getenv("APP_ORIGIN"),
		CookieSecure:    true,
		SessionTTL:      30 * 24 * time.Hour,
		FileStorageDir:  getenv("FILE_STORAGE_DIR"),
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.AppOrigin == "" {
		return Config{}, errors.New("APP_ORIGIN is required, e.g. http://localhost:5180")
	}
	if err := validateOrigin(cfg.AppOrigin); err != nil {
		return Config{}, fmt.Errorf("APP_ORIGIN: %w", err)
	}
	if cfg.FileStorageDir == "" {
		return Config{}, errors.New("FILE_STORAGE_DIR is required, e.g. ./data/files")
	}
	if raw := getenv("COOKIE_SECURE"); raw != "" {
		secure, err := strconv.ParseBool(raw)
		if err != nil {
			return Config{}, fmt.Errorf("COOKIE_SECURE must be true or false, got %q", raw)
		}
		cfg.CookieSecure = secure
	}
	return cfg, nil
}

func validateOrigin(origin string) error {
	u, err := url.Parse(origin)
	if err != nil {
		return err
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("must look like https://example.com with no path, got %q", origin)
	}
	return nil
}
