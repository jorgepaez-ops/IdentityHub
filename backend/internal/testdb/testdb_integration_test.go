//go:build integration

package testdb_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRNF011_MigracionesSeAplicanSobreBaseVacia(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}

	pool := testdb.New(t)
	ctx := context.Background()

	for _, table := range []string{
		"users",
		"roles",
		"user_roles",
		"refresh_tokens",
		"verification_tokens",
		"recovery_codes",
		"audit_log",
	} {
		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = 'public' AND tablename = $1)", table).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %q: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %q to exist", table)
		}
	}

	_, err := pool.Exec(ctx, "INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)", "md5@example.test", "d41d8cd98f00b204e9800998ecf8427e", "MD5 user")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("insert MD5 password hash error = %v, want PostgreSQL constraint error", err)
	}
	if pgErr.Code != "23514" {
		t.Errorf("MD5 constraint error code = %q, want 23514", pgErr.Code)
	}
	if pgErr.ConstraintName != "users_password_hash_is_argon2id" {
		t.Errorf("MD5 constraint name = %q, want users_password_hash_is_argon2id", pgErr.ConstraintName)
	}

	_, err = pool.Exec(ctx, "INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)", "argon@example.test", "$argon2id$fixed-test-value", "Argon2id user")
	if err != nil {
		t.Fatalf("insert Argon2id password hash: %v", err)
	}
}
