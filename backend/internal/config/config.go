// Package config reads process configuration from environment variables.
package config

import (
	"errors"
	"time"
)

// Config holds settings the API server needs at startup.
type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	ShutdownTimeout time.Duration
}

// Load builds a Config from the given lookup function (normally os.Getenv).
// Passing the lookup in keeps Load deterministic in tests.
func Load(getenv func(string) string) (Config, error) {
	cfg := Config{
		HTTPAddr:        getenv("HTTP_ADDR"),
		DatabaseURL:     getenv("DATABASE_URL"),
		ShutdownTimeout: 10 * time.Second,
	}
	if cfg.HTTPAddr == "" {
		cfg.HTTPAddr = ":8080"
	}
	if cfg.DatabaseURL == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	return cfg, nil
}
