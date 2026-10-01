//go:build integration

package refresh_test

import (
	"context"
	"testing"
	"time"

	"errors"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/lockout"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/refresh"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
	"github.com/jorgepaez/identity-hub/internal/testsession"
)

func TestRF006_DosRenovacionesConcurrentesUnaGana(t *testing.T) {
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
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "refresh-race@example.test", PasswordHash: passwordHash, DisplayName: "Refresh race"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, user.ID); err != nil {
		t.Fatalf("activate user: %v", err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	loginResult, err := testsession.New(repository, signer, lockout.Default()).SignIn(ctx, login.Input{Email: user.Email, Password: "correct horse battery"})
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	service := refresh.New(repository, signer, time.Hour)
	var familyID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT family_id FROM refresh_tokens WHERE user_id = $1 LIMIT 1`, user.ID).Scan(&familyID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO hub_sessions (user_id, token_hash, family_id, expires_at) VALUES ($1,$2,$3,$4)`, user.ID, make([]byte, 32), familyID, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for range 2 {
		go func() {
			<-start
			_, err := service.Refresh(ctx, refresh.Input{RefreshToken: loginResult.RefreshToken})
			results <- err
		}()
	}
	close(start)
	var success, reuse int
	for range 2 {
		if err := <-results; err == nil {
			success++
		} else if errors.Is(err, refresh.ErrRefreshReuse) {
			reuse++
		} else {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if success != 1 || reuse != 1 {
		t.Fatalf("success=%d reuse=%d", success, reuse)
	}
	var activeCount, auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM refresh_tokens WHERE user_id = $1 AND status = 'active'`, user.ID).Scan(&activeCount); err != nil {
		t.Fatalf("count active refresh tokens: %v", err)
	}
	if activeCount != 0 {
		t.Errorf("active refresh tokens = %d, want 0 after reuse", activeCount)
	}
	var hubRevoked bool
	if err := pool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM hub_sessions WHERE family_id = $1`, familyID).Scan(&hubRevoked); err != nil || !hubRevoked {
		t.Fatalf("hub session revoked=%t err=%v", hubRevoked, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE actor_user_id = $1 AND action = 'refresh_reuse_detected'`, user.ID).Scan(&auditCount); err != nil {
		t.Fatalf("count reuse audits: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("refresh_reuse_detected audits = %d, want 1", auditCount)
	}
}
