//go:build integration

package password

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRF001_UsersAceptaArgon2idYRechazaMD5(t *testing.T) {
	pool := testdb.New(t)
	hash, err := Hash("integration-fixed-password")
	if err != nil {
		t.Fatalf("Hash() error = %v", err)
	}
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)`, "argon2id@example.test", hash, "Argon2id user"); err != nil {
		t.Fatalf("insert Argon2id hash: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO users (email, password_hash, display_name) VALUES ($1, $2, $3)`, "md5@example.test", strings.Repeat("a", 32), "MD5 user")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("insert MD5-shaped hash error = %v, want a PostgreSQL constraint error", err)
	}
	if pgErr.Code != "23514" || pgErr.ConstraintName != "users_password_hash_is_argon2id" {
		t.Fatalf("insert MD5-shaped hash: code %q constraint %q, want 23514 users_password_hash_is_argon2id", pgErr.Code, pgErr.ConstraintName)
	}
}
