//go:build integration

package login_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/netip"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/auth/lockout"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
	"github.com/jorgepaez/identity-hub/internal/testsession"
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
	session := testsession.New(repository, signer, lockout.Default())
	ip := netip.MustParseAddr("203.0.113.20")
	userAgent := "Identity Hub integration"
	input := login.Input{Email: "login-integration@example.test", Password: "correct horse battery", IP: &ip, UserAgent: &userAgent}

	// The password step alone is not a login: no success audit, no last_login_at.
	if _, err := session.Login.Login(ctx, input); err != nil {
		t.Fatalf("Login: %v", err)
	}
	var lastLoginAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT last_login_at FROM users WHERE id = $1`, user.ID).Scan(&lastLoginAt); err != nil {
		t.Fatalf("read last_login_at: %v", err)
	}
	var succeededBeforeCode int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE actor_user_id = $1 AND action = 'login_succeeded'`, user.ID).Scan(&succeededBeforeCode); err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if lastLoginAt != nil || succeededBeforeCode != 0 {
		t.Fatalf("password step recorded a login: last_login_at=%v login_succeeded=%d", lastLoginAt, succeededBeforeCode)
	}

	result, err := session.SignIn(ctx, input)
	if err != nil {
		t.Fatalf("SignIn: %v", err)
	}
	if result.AccessToken == "" || result.RefreshToken == "" {
		t.Fatalf("result = %+v", result)
	}

	rawRefreshToken, err := base64.RawURLEncoding.DecodeString(result.RefreshToken)
	if err != nil {
		t.Fatalf("decode refresh token: %v", err)
	}
	wantHash := sha256.Sum256(rawRefreshToken)
	var storedHash []byte
	if err := pool.QueryRow(ctx, `SELECT token_hash FROM refresh_tokens WHERE user_id = $1`, user.ID).Scan(&storedHash); err != nil {
		t.Fatalf("read refresh token: %v", err)
	}
	if string(storedHash) != string(wantHash[:]) {
		t.Error("stored refresh token hash does not match SHA-256(refreshToken)")
	}

	lastLoginAt = nil
	if err := pool.QueryRow(ctx, `SELECT last_login_at FROM users WHERE id = $1`, user.ID).Scan(&lastLoginAt); err != nil {
		t.Fatalf("read last_login_at: %v", err)
	}
	if lastLoginAt == nil {
		t.Error("last_login_at was not set")
	}

	var auditCount int
	var auditIPText string
	var auditUserAgent string
	if err := pool.QueryRow(ctx, `SELECT count(*), max(host(ip)::text), max(user_agent) FROM audit_log WHERE actor_user_id = $1 AND action = 'login_succeeded'`, user.ID).Scan(&auditCount, &auditIPText, &auditUserAgent); err != nil {
		t.Fatalf("read audit log: %v", err)
	}
	if auditCount != 1 || auditIPText != ip.String() || auditUserAgent != userAgent {
		t.Errorf("login_succeeded audit = count %d ip %q user-agent %q", auditCount, auditIPText, auditUserAgent)
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
	service := testsession.New(repository, signer, lockout.Config{AccountMaxFailures: 1, IPMaxFailures: 20, FailureWindow: 15 * time.Minute, LockoutDuration: 15 * time.Minute}).Login
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
