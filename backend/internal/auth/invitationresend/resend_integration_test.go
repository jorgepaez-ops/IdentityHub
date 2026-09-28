//go:build integration

package invitationresend

import (
	"bytes"
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

type integrationPublisher struct{}

func (integrationPublisher) Publish(context.Context, string, any) error { return nil }

func TestRF001_ReenvioInvalidaLaInvitacionAnterior(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "resend-integration@example.test", PasswordHash: "$argon2id$placeholder", DisplayName: "Resend Integration"})
	if err != nil {
		t.Fatal(err)
	}
	actor, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "resend-actor@example.test", PasswordHash: "$argon2id$placeholder", DisplayName: "Resend Actor"})
	if err != nil {
		t.Fatal(err)
	}
	old := sha256.Sum256([]byte("old-invitation-token"))
	if _, err := pool.Exec(ctx, `INSERT INTO verification_tokens (user_id,token_hash,purpose,expires_at) VALUES ($1,$2,'invitation',$3)`, user.ID, old[:], time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	service := New(repository, integrationPublisher{}, bytes.NewReader(bytes.Repeat([]byte{9}, 32)), time.Now)
	if err := service.Resend(ctx, Input{ActorUserID: actor.ID, UserID: user.ID}); err != nil {
		t.Fatal(err)
	}
	var usedAt *time.Time
	if err := pool.QueryRow(ctx, `SELECT used_at FROM verification_tokens WHERE token_hash=$1`, old[:]).Scan(&usedAt); err != nil {
		t.Fatal(err)
	}
	if usedAt == nil {
		t.Fatal("old invitation remained usable")
	}
	var usable int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM verification_tokens WHERE user_id=$1 AND purpose='invitation' AND used_at IS NULL AND expires_at > now()`, user.ID).Scan(&usable); err != nil {
		t.Fatal(err)
	}
	if usable != 1 {
		t.Fatalf("usable invitations=%d want 1", usable)
	}
}
