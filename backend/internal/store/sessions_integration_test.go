//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
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
	owner, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "sessions-owner@example.test", PasswordHash: "$argon2id$fixed-test-value", DisplayName: "Owner"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "sessions-other@example.test", PasswordHash: "$argon2id$fixed-test-value", DisplayName: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	ownerFamily, otherFamily := uuid.New(), uuid.New()
	for _, item := range []struct {
		userID, familyID uuid.UUID
		hash             byte
	}{
		{owner.ID, ownerFamily, 1},
		{other.ID, otherFamily, 2},
	} {
		tokenHash := make([]byte, 32)
		for index := range tokenHash {
			tokenHash[index] = item.hash
		}
		if _, err := pool.Exec(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, family_id, ip, user_agent, expires_at) VALUES ($1, $2, $3, '203.0.113.8', 'integration client', $4)`, item.userID, tokenHash, item.familyID, time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	for _, item := range []struct {
		userID, familyID uuid.UUID
		hash             byte
	}{
		{owner.ID, ownerFamily, 3},
		{other.ID, otherFamily, 4},
	} {
		tokenHash := make([]byte, 32)
		for index := range tokenHash {
			tokenHash[index] = item.hash
		}
		if _, err := pool.Exec(ctx, `INSERT INTO hub_sessions (user_id, token_hash, family_id, expires_at) VALUES ($1, $2, $3, $4)`, item.userID, tokenHash, item.familyID, time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}

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
	var revokedOwnerHub, revokedOtherHub bool
	if err := pool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM hub_sessions WHERE family_id = $1`, ownerFamily).Scan(&revokedOwnerHub); err != nil || !revokedOwnerHub {
		t.Fatalf("owner hub session revoked=%t err=%v", revokedOwnerHub, err)
	}
	if err := pool.QueryRow(ctx, `SELECT revoked_at IS NOT NULL FROM hub_sessions WHERE family_id = $1`, otherFamily).Scan(&revokedOtherHub); err != nil || revokedOtherHub {
		t.Fatalf("other hub session revoked=%t err=%v", revokedOtherHub, err)
	}
	items, err = service.List(ctx, owner.ID)
	if err != nil || len(items) != 0 {
		t.Fatalf("items after revoke=%+v err=%v", items, err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE actor_user_id = $1 AND action = 'session_revoked' AND resource_id = $2`, owner.ID, ownerFamily.String()).Scan(&count); err != nil || count != 1 {
		t.Fatalf("audit count=%d err=%v", count, err)
	}
}
