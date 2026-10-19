//go:build integration

package refresh_test

import (
	"context"
	"testing"
	"time"

	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
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
	user, err := repository.CreateUser(ctx, store.CreateUserParams{OrganizationID: store.ContabilidadOrganizationID, Email: "refresh-race@example.test", PasswordHash: passwordHash, DisplayName: "Refresh race"})
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
	success, reuse := refreshConcurrently(t, service, loginResult.RefreshToken, 2)
	if success != 1 || reuse != 1 {
		t.Fatalf("success=%d reuse=%d", success, reuse)
	}
	requireReuseRevokedTheFamily(t, pool, user.ID, familyID)
}

// refreshConcurrently releases n goroutines at once against the same refresh token and tallies
// the successes and the reuse rejections; any other error fails the test.
func refreshConcurrently(t *testing.T, service *refresh.Service, refreshToken string, n int) (success, reuse int) {
	t.Helper()
	start := make(chan struct{})
	results := make(chan error, n)
	for range n {
		go func() {
			<-start
			_, err := service.Refresh(context.Background(), refresh.Input{RefreshToken: refreshToken})
			results <- err
		}()
	}
	close(start)
	for range n {
		err := <-results
		switch {
		case err == nil:
			success++
		case errors.Is(err, refresh.ErrRefreshReuse):
			reuse++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}
	return success, reuse
}

// requireReuseRevokedTheFamily checks the consequences of a detected reuse: no active refresh
// token left, the hub session of the family revoked and exactly one reuse audit event.
func requireReuseRevokedTheFamily(t *testing.T, pool *pgxpool.Pool, userID, familyID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	var activeCount, auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM refresh_tokens WHERE user_id = $1 AND status = 'active'`, userID).Scan(&activeCount); err != nil {
		t.Fatalf("count active refresh tokens: %v", err)
	}
	if activeCount != 0 {
		t.Errorf("active refresh tokens = %d, want 0 after reuse", activeCount)
	}
	var hubRevoked bool
	if err := pool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM hub_sessions WHERE family_id = $1`, familyID).Scan(&hubRevoked); err != nil || !hubRevoked {
		t.Fatalf("hub session revoked=%t err=%v", hubRevoked, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE actor_user_id = $1 AND action = 'refresh_reuse_detected'`, userID).Scan(&auditCount); err != nil {
		t.Fatalf("count reuse audits: %v", err)
	}
	if auditCount != 1 {
		t.Errorf("refresh_reuse_detected audits = %d, want 1", auditCount)
	}
}
