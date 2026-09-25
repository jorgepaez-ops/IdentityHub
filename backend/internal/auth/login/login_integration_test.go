//go:build integration

package login_test

import (
	"context"
	"crypto/sha256"
	"errors"
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

func TestRF017_BloqueoSePersisteEnPostgres(t *testing.T) {
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
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "lockout-integration@example.test", PasswordHash: hash, DisplayName: "Lockout test"})
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
	service := login.New(repository, signer, time.Hour, login.LockoutConfig{AccountMaxFailures: 1, IPMaxFailures: 20, FailureWindow: 15 * time.Minute, LockoutDuration: 15 * time.Minute})
	if _, err := service.Login(ctx, login.Input{Email: user.Email, Password: "wrong password"}); !errors.Is(err, login.ErrInvalidCredentials) {
		t.Fatalf("failed login error = %v", err)
	}
	if _, err := service.Login(ctx, login.Input{Email: user.Email, Password: "correct horse battery"}); !errors.Is(err, login.ErrAccountLocked) {
		t.Fatalf("locked correct-password login error = %v", err)
	}
	var status string
	var lockedUntil *time.Time
	if err := pool.QueryRow(ctx, `SELECT status, locked_until FROM users WHERE id = $1`, user.ID).Scan(&status, &lockedUntil); err != nil {
		t.Fatalf("read persisted lock: %v", err)
	}
	if status != "locked" || lockedUntil == nil || !lockedUntil.After(time.Now().Add(14*time.Minute)) {
		t.Fatalf("persisted lock = status=%q locked_until=%v", status, lockedUntil)
	}
	var indexExists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_indexes WHERE schemaname = 'public' AND indexname = 'audit_log_ip_created_idx')`).Scan(&indexExists); err != nil {
		t.Fatalf("read audit index: %v", err)
	}
	if !indexExists {
		t.Fatal("audit_log_ip_created_idx was not created")
	}
}
