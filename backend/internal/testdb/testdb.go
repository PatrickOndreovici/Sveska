//go:build integration

// Package testdb connects integration tests to the dedicated test database.
//
// Run with `make test-integration`, which creates and migrates the database
// named by POSTGRES_TEST_DB and exports TEST_DATABASE_URL.
package testdb

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// New returns a pool for the test database with every application table
// emptied, so each test starts from a clean state. Tests using it must not
// run in parallel with each other (`go test -p 1`).
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Fatal("TEST_DATABASE_URL is not set; run integration tests with `make test-integration`")
	}
	if !strings.Contains(url, "test") {
		t.Fatalf("refusing to truncate a database whose URL does not contain \"test\": %s", url)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	t.Cleanup(pool.Close)

	var tables string
	err = pool.QueryRow(ctx, `
		SELECT coalesce(string_agg(format('%I', tablename), ', '), '')
		FROM pg_tables
		WHERE schemaname = 'public' AND tablename <> 'schema_migrations'`).Scan(&tables)
	if err != nil {
		t.Fatalf("list tables: %v", err)
	}
	if tables != "" {
		if _, err := pool.Exec(ctx, "TRUNCATE "+tables+" CASCADE"); err != nil {
			t.Fatalf("truncate tables: %v", err)
		}
	}
	return pool
}
