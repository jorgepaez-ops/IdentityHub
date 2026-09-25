//go:build integration

package logout_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/logout"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/refresh"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRF007_RefreshTrasLogoutDevuelve401(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	ctx := context.Background()
	passwordHash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "logout-refresh@example.test", PasswordHash: passwordHash, DisplayName: "Logout refresh"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := pool.Exec(ctx, "UPDATE users SET status = 'active' WHERE id = $1", user.ID); err != nil {
		t.Fatalf("activate user: %v", err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	loginResult, err := login.New(repository, signer, time.Hour).Login(ctx, login.Input{Email: user.Email, Password: "correct horse battery"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if err := logout.New(repository).Logout(ctx, logout.Input{RefreshToken: loginResult.RefreshToken}); err != nil {
		t.Fatalf("Logout: %v", err)
	}
	if _, err := refresh.New(repository, signer, time.Hour).Refresh(ctx, refresh.Input{RefreshToken: loginResult.RefreshToken}); !errors.Is(err, refresh.ErrInvalidRefreshToken) {
		t.Fatalf("Refresh error=%v, want ErrInvalidRefreshToken", err)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM audit_log WHERE actor_user_id = $1 AND action = 'logout'", user.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count logout audits: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("logout audits=%d, want 1", auditCount)
	}
}
