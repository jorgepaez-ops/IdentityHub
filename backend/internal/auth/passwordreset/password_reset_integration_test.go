//go:build integration

package passwordreset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"

	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

type integrationPublisher struct{}

func (integrationPublisher) Publish(context.Context, string, any) error { return nil }

type integrationHasher struct{}

func (integrationHasher) Hash(value string) (string, error) { return password.Hash(value) }

func TestRF015_RestablecimientoRevocaTodasLasSesiones(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "reset-revocation@example.test", PasswordHash: "$argon2id$placeholder", DisplayName: "Reset Revocation"})
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("password-reset-integration-token")
	hash := sha256.Sum256(raw)
	if _, err := pool.Exec(ctx, `INSERT INTO verification_tokens (user_id, token_hash, purpose, expires_at) VALUES ($1,$2,'password_reset',$3)`, user.ID, hash[:], time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	for index := range 2 {
		refreshHash := sha256.Sum256([]byte{byte(index)})
		if _, err := pool.Exec(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, family_id, expires_at) VALUES ($1,$2,$3,$4)`, user.ID, refreshHash[:], uuid.New(), time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	service := New(repository, integrationPublisher{}, integrationHasher{}, bytes.NewReader(make([]byte, 32)), time.Now)
	if err := service.Confirm(ctx, base64.RawURLEncoding.EncodeToString(raw), "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	if err := service.Confirm(ctx, base64.RawURLEncoding.EncodeToString(raw), "correct horse battery"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("second reset use error=%v, want ErrTokenInvalid", err)
	}
	var active int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM refresh_tokens WHERE user_id=$1 AND status='active'`, user.ID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("active refresh tokens=%d want 0", active)
	}
	var passwordHash string
	if err := pool.QueryRow(ctx, `SELECT password_hash FROM users WHERE id=$1`, user.ID).Scan(&passwordHash); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(passwordHash, "$argon2id$") {
		t.Fatalf("password hash=%q", passwordHash)
	}
}

func TestRF015_ConsumirRestablecimientoInvalidaLosDemasTokens(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "reset-other-token@example.test", PasswordHash: "$argon2id$placeholder", DisplayName: "Reset Other Token"})
	if err != nil {
		t.Fatal(err)
	}
	firstRaw := []byte("password-reset-first-token")
	secondRaw := []byte("password-reset-second-token")
	for _, raw := range [][]byte{firstRaw, secondRaw} {
		hash := sha256.Sum256(raw)
		if _, err := pool.Exec(ctx, `INSERT INTO verification_tokens (user_id, token_hash, purpose, expires_at) VALUES ($1,$2,'password_reset',$3)`, user.ID, hash[:], time.Now().Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	service := New(repository, integrationPublisher{}, integrationHasher{}, bytes.NewReader(make([]byte, 32)), time.Now)
	if err := service.Confirm(ctx, base64.RawURLEncoding.EncodeToString(firstRaw), "correct horse battery"); err != nil {
		t.Fatal(err)
	}
	if err := service.Confirm(ctx, base64.RawURLEncoding.EncodeToString(secondRaw), "correct horse battery"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("second reset token error=%v, want ErrTokenInvalid", err)
	}
}

func TestRF015_SolicitudAusenteNoPersisteTokenUtil(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	service := New(repository, integrationPublisher{}, integrationHasher{}, bytes.NewReader(bytes.Repeat([]byte{5}, 32)), time.Now)
	const email = "not-registered-reset@example.test"
	if err := service.Request(context.Background(), email); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM verification_tokens WHERE purpose='password_reset'`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("usable reset tokens=%d want 0 for absent email", count)
	}
}
