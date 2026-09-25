//go:build integration

package login_test

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRF003_LoginPersisteRefreshTokenYAuditoriaEnPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	ctx := context.Background()

	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "login-integration@example.test", PasswordHash: hash, DisplayName: "Login test"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, user.ID); err != nil {
		t.Fatalf("activate user: %v", err)
	}

	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatalf("token.New: %v", err)
	}
	service := login.New(repository, signer, time.Hour)

	result, err := service.Login(ctx, login.Input{Email: "login-integration@example.test", Password: "correct horse battery"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		t.Fatalf("result = %+v", result)
	}

	wantHash := sha256.Sum256([]byte(result.RefreshToken))
	var storedHash []byte
	if err := pool.QueryRow(ctx, `SELECT token_hash FROM refresh_tokens WHERE user_id = $1`, user.ID).Scan(&storedHash); err != nil {
		t.Fatalf("read refresh token: %v", err)
	}
	if string(storedHash) != string(wantHash[:]) {
		t.Error("stored refresh token hash does not match SHA-256(refreshToken)")
	}

	var lastLoginAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT last_login_at FROM users WHERE id = $1`, user.ID).Scan(&lastLoginAt); err != nil {
		t.Fatalf("read last_login_at: %v", err)
	}
	if lastLoginAt == nil {
		t.Error("last_login_at was not set")
	}

	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE actor_user_id = $1 AND action = 'login_succeeded'`, user.ID).Scan(&auditCount); err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("audit login_succeeded count = %d, want 1", auditCount)
	}
}
