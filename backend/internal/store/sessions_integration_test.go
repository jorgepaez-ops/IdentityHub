//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jorgepaez/identity-hub/internal/auth/session"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRF016_ListarYRevocarUnaFamiliaSinAfectarOtra(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := repository.CreateUser(ctx, store.CreateUserParams{OrganizationID: store.ContabilidadOrganizationID, Email: "sessions-owner@example.test", PasswordHash: "$argon2id$fixed-test-value", DisplayName: "Owner"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := repository.CreateUser(ctx, store.CreateUserParams{OrganizationID: store.ContabilidadOrganizationID, Email: "sessions-other@example.test", PasswordHash: "$argon2id$fixed-test-value", DisplayName: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	ownerFamily, otherFamily := uuid.New(), uuid.New()
	insertTokenRows(t, pool, `INSERT INTO refresh_tokens (user_id, token_hash, family_id, ip, user_agent, expires_at) VALUES ($1, $2, $3, '203.0.113.8', 'integration client', $4)`, []tokenSeed{
		{owner.ID, ownerFamily, 1},
		{other.ID, otherFamily, 2},
	})
	insertTokenRows(t, pool, `INSERT INTO hub_sessions (user_id, token_hash, family_id, expires_at) VALUES ($1, $2, $3, $4)`, []tokenSeed{
		{owner.ID, ownerFamily, 3},
		{other.ID, otherFamily, 4},
	})

	service := session.New(repository)
	items, err := service.List(ctx, owner.ID)
	if err != nil || len(items) != 1 || items[0].ID != ownerFamily {
		t.Fatalf("items=%+v err=%v", items, err)
	}
	if err := service.Revoke(ctx, owner.ID, otherFamily); !errors.Is(err, session.ErrSessionNotFound) {
		t.Fatalf("foreign revoke err=%v, want ErrSessionNotFound", err)
	}
	if err := service.Revoke(ctx, owner.ID, ownerFamily); err != nil {
		t.Fatal(err)
	}
	requireHubSessionRevoked(t, pool, "owner", ownerFamily, true)
	requireHubSessionRevoked(t, pool, "other", otherFamily, false)
	items, err = service.List(ctx, owner.ID)
	if err != nil || len(items) != 0 {
		t.Fatalf("items after revoke=%+v err=%v", items, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE actor_user_id = $1 AND action = 'session_revoked' AND resource_id = $2`, owner.ID, ownerFamily.String()).Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
}

// tokenSeed describes one refresh-token or hub-session row: its owner, family and the byte that
// fills its 32-byte token hash.
type tokenSeed struct {
	userID, familyID uuid.UUID
	hash             byte
}

// insertTokenRows runs insert (user_id, token_hash, family_id, expires_at) once per seed with
// a token hash filled with the seed byte and an expiry one hour ahead.
func insertTokenRows(t *testing.T, pool *pgxpool.Pool, insert string, seeds []tokenSeed) {
	t.Helper()
	for _, item := range seeds {
		tokenHash := make([]byte, 32)
		for index := range tokenHash {
			tokenHash[index] = item.hash
		}
		if _, err := pool.Exec(context.Background(), insert, item.userID, tokenHash, item.familyID, time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
}

// requireHubSessionRevoked checks whether the hub session of the family is revoked as expected.
func requireHubSessionRevoked(t *testing.T, pool *pgxpool.Pool, owner string, familyID uuid.UUID, want bool) {
	t.Helper()
	var revoked bool
	err := pool.QueryRow(context.Background(), `SELECT revoked_at IS NOT NULL FROM hub_sessions WHERE family_id = $1`, familyID).Scan(&revoked)
	if err != nil || revoked != want {
		t.Fatalf("%s hub session revoked=%t err=%v, want revoked=%t", owner, revoked, err, want)
	}
}
