//go:build integration

package verification

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

type integrationPublisher struct{}

func (integrationPublisher) Publish(context.Context, string, any) error { return nil }

func TestRF002_VerificarConsumeTokenYActivaCuenta(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	ctx := context.Background()
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "verification@example.test", PasswordHash: "$argon2id$integration-test", DisplayName: "Verification test"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	rawToken := "one-time-email-token"
	tokenHash := sha256.Sum256([]byte(rawToken))
	err = repository.WithinRegistrationTransaction(ctx, func(writer store.RegistrationWriter) error {
		return writer.CreateVerificationToken(ctx, store.CreateVerificationTokenParams{UserID: user.ID, TokenHash: tokenHash[:], ExpiresAt: time.Now().Add(time.Hour)})
	})
	if err != nil {
		t.Fatalf("CreateVerificationToken: %v", err)
	}

	service := New(repository, integrationPublisher{})
	if err := service.Verify(ctx, Input{Token: rawToken}); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	active, err := repository.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if active.Status != "active" {
		t.Fatalf("status = %q, want active", active.Status)
	}
	if err := service.Verify(ctx, Input{Token: rawToken}); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("second Verify() error = %v, want ErrTokenInvalid", err)
	}
}
