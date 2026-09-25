//go:build integration

package registration

import (
	"context"
	"crypto/rand"
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

func TestRF001_RegistroPersisteHashArgon2id(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	service := New(repository, integrationPublisher{}, passwordHasher{}, rand.Reader, time.Now)
	_, err = service.Register(context.Background(), Input{Email: "registration-argon2id@example.test", Password: "correct horse battery", DisplayName: "Registration test"})
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	var hash string
	if err := pool.QueryRow(context.Background(), `SELECT password_hash FROM users WHERE email = $1`, "registration-argon2id@example.test").Scan(&hash); err != nil {
		t.Fatalf("read password hash: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("password_hash = %q, want Argon2id PHC prefix", hash)
	}
}
