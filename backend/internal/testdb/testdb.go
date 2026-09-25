//go:build integration

// Package testdb provisions disposable PostgreSQL databases for integration tests.
package testdb

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// New creates a database dedicated to one test, applies every up migration, and
// registers cleanup that closes connections before dropping the database.
func New(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for integration tests")
	}

	adminConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}

	ctx := context.Background()
	adminPool, err := pgxpool.NewWithConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("connect integration database server: %v", err)
	}

	databaseName := temporaryDatabaseName(t)
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+databaseIdentifier(databaseName)); err != nil {
		adminPool.Close()
		t.Fatalf("create temporary database: %v", err)
	}

	testConfig := adminConfig.Copy()
	testConfig.ConnConfig.Database = databaseName
	testPool, err := pgxpool.NewWithConfig(ctx, testConfig)
	if err != nil {
		dropDatabase(ctx, adminPool, databaseName)
		adminPool.Close()
		t.Fatalf("connect temporary database: %v", err)
	}

	t.Cleanup(func() {
		testPool.Close()
		if err := dropDatabase(ctx, adminPool, databaseName); err != nil {
			t.Errorf("drop temporary database: %v", err)
		}
		adminPool.Close()
	})

	applyMigrations(t, ctx, testPool)
	return testPool
}

func temporaryDatabaseName(t *testing.T) string {
	t.Helper()

	suffix := make([]byte, 12)
	if _, err := rand.Read(suffix); err != nil {
		t.Fatalf("generate temporary database suffix: %v", err)
	}
	return "identity_test_" + hex.EncodeToString(suffix)
}

func applyMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(migrationsDirectory(t), "*.up.sql"))
	if err != nil {
		t.Fatalf("find migration files: %v", err)
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatal("no up migration files found")
	}

	for _, file := range files {
		sql, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read migration %s: %v", filepath.Base(file), err)
		}
		// A migration file holds several statements, which only PostgreSQL's simple
		// protocol accepts in one call. Only this Exec uses it: the pool keeps pgx's
		// default (extended) protocol so tests exercise the same path as production.
		if _, err := pool.Exec(ctx, string(sql), pgx.QueryExecModeSimpleProtocol); err != nil {
			t.Fatalf("apply migration %s: %v", filepath.Base(file), err)
		}
	}
}

func migrationsDirectory(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate testdb source file")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "db", "migrations")
}

func dropDatabase(ctx context.Context, adminPool *pgxpool.Pool, databaseName string) error {
	if _, err := adminPool.Exec(ctx, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()", databaseName); err != nil {
		return fmt.Errorf("terminate connections: %w", err)
	}
	if _, err := adminPool.Exec(ctx, "DROP DATABASE IF EXISTS "+databaseIdentifier(databaseName)); err != nil {
		return fmt.Errorf("drop database: %w", err)
	}
	return nil
}

func databaseIdentifier(databaseName string) string {
	// PostgreSQL does not support parameters for DDL identifiers. The name is
	// generated locally from a fixed prefix and crypto/rand, then quoted by pgx.
	return pgx.Identifier{databaseName}.Sanitize()
}
