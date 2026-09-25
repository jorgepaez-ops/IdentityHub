//go:build integration

package verification

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
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
	// Mirror registration.Service exactly: hash the raw bytes for storage,
	// base64url-encode the same raw bytes for what actually goes in the
	// email link. Hashing the encoded string here (instead of decoding it
	// first) is the real bug this test now guards against.
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		t.Fatalf("generate raw token: %v", err)
	}
	tokenHash := sha256.Sum256(rawToken)
	emailedToken := base64.RawURLEncoding.EncodeToString(rawToken)
	err = repository.WithinRegistrationTransaction(ctx, func(writer store.RegistrationWriter) error {
		return writer.CreateVerificationToken(ctx, store.CreateVerificationTokenParams{UserID: user.ID, TokenHash: tokenHash[:], ExpiresAt: time.Now().Add(time.Hour)})
	})
	if err != nil {
		t.Fatalf("CreateVerificationToken: %v", err)
	}

	service := New(repository, integrationPublisher{})
	if err := service.Verify(ctx, Input{Token: emailedToken}); err != nil {
		t.Fatalf("Verify: %v", err)
	}
	active, err := repository.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if active.Status != "active" {
		t.Fatalf("status = %q, want active", active.Status)
	}
	if err := service.Verify(ctx, Input{Token: emailedToken}); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("second Verify() error = %v, want ErrTokenInvalid", err)
	}
}
