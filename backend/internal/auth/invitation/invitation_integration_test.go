//go:build integration

package invitation

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

type integrationPublisher struct{}

func (integrationPublisher) Publish(context.Context, string, any) error { return nil }

type passwordHasher struct{}

func (passwordHasher) Hash(value string) (string, error) { return password.Hash(value) }

func TestRF002_AceptarPersisteHashArgon2idYActivaLaCuenta(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	ctx := context.Background()
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "invitation-integration@example.test", PasswordHash: "$argon2id$placeholder-integration-test", DisplayName: "Invitation integration"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		t.Fatalf("generate raw token: %v", err)
	}
	tokenHash := sha256.Sum256(rawToken)
	emailedToken := base64.RawURLEncoding.EncodeToString(rawToken)
	err = repository.WithinEmployeeCreationTransaction(ctx, func(writer store.EmployeeCreationWriter) error {
		return writer.CreateInvitationToken(ctx, store.CreateInvitationTokenParams{UserID: user.ID, TokenHash: tokenHash[:], ExpiresAt: time.Now().Add(24 * time.Hour)})
	})
	if err != nil {
		t.Fatalf("CreateInvitationToken: %v", err)
	}

	service := New(repository, integrationPublisher{}, passwordHasher{})
	if err := service.Accept(ctx, Input{Token: emailedToken, Password: "correct horse battery"}); err != nil {
		t.Fatalf("Accept: %v", err)
	}

	var hash, status string
	if err := pool.QueryRow(ctx, `SELECT password_hash, status FROM users WHERE id = $1`, user.ID).Scan(&hash, &status); err != nil {
		t.Fatalf("read activated user: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("password_hash = %q, want Argon2id PHC prefix", hash)
	}
	if hash == "$argon2id$placeholder-integration-test" {
		t.Error("password_hash was not replaced by the invitee's chosen password")
	}
	if status != "active" {
		t.Errorf("status = %q, want active", status)
	}

	if err := service.Accept(ctx, Input{Token: emailedToken, Password: "correct horse battery"}); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("second Accept() error = %v, want ErrTokenInvalid (invitations are single-use, RF-002)", err)
	}
}

// The invitation purpose must never accept an email-verification token, even
// though they share the same verification_tokens table (invariant 7).
func TestRF002_UnTokenDeVerificacionDeCorreoNoActivaPorInvitacion(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	ctx := context.Background()
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "wrong-purpose@example.test", PasswordHash: "$argon2id$placeholder", DisplayName: "Wrong purpose"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
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

	service := New(repository, integrationPublisher{}, passwordHasher{})
	if err := service.Accept(ctx, Input{Token: emailedToken, Password: "correct horse battery"}); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Accept() error = %v, want ErrTokenInvalid for an email_verification-purpose token", err)
	}
}
