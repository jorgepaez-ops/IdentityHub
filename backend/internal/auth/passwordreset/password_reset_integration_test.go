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
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/mfa"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

type integrationPublisher struct{}

func (integrationPublisher) Publish(context.Context, string, any) error { return nil }

type integrationHasher struct{}

func (integrationHasher) Hash(value string) (string, error) { return password.Hash(value) }

type integrationMFAIssuer struct{}

func (integrationMFAIssuer) Issue(context.Context, mfa.User) (mfa.Challenge, error) {
	return mfa.Challenge{Token: "integration-mfa-challenge", ExpiresIn: 300}, nil
}

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

func newIntegrationLoginService(t *testing.T, repository *store.Store) *login.Service {
	t.Helper()
	return login.New(repository).WithMFA(integrationMFAIssuer{})
}

// D13: a reset completed on a locked account (RF-017) reactivates it and the
// new password works immediately.
func TestRF015_D13_RestablecerDesbloqueaYPermiteEntrarConLaNuevaContrasena(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "d13-locked@example.test", PasswordHash: "$argon2id$placeholder", DisplayName: "D13 Locked"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'locked', locked_until = $2 WHERE id = $1`, user.ID, time.Now().Add(15*time.Minute)); err != nil {
		t.Fatalf("lock user: %v", err)
	}
	raw := []byte("d13-locked-reset-token")
	hash := sha256.Sum256(raw)
	if _, err := pool.Exec(ctx, `INSERT INTO verification_tokens (user_id, token_hash, purpose, expires_at) VALUES ($1,$2,'password_reset',$3)`, user.ID, hash[:], time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	service := New(repository, integrationPublisher{}, integrationHasher{}, bytes.NewReader(make([]byte, 32)), time.Now)
	if err := service.Confirm(ctx, base64.RawURLEncoding.EncodeToString(raw), "correct horse battery"); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	var status string
	var lockedUntil *time.Time
	if err := pool.QueryRow(ctx, `SELECT status, locked_until FROM users WHERE id=$1`, user.ID).Scan(&status, &lockedUntil); err != nil {
		t.Fatal(err)
	}
	if status != "active" || lockedUntil != nil {
		t.Fatalf("status=%q locked_until=%v, want active/nil", status, lockedUntil)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE actor_user_id=$1 AND action='password_reset_completed'`, user.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("password_reset_completed audits=%d want 1", auditCount)
	}
	var unlocked string
	if err := pool.QueryRow(ctx, `SELECT metadata ->> 'unlocked' FROM audit_log WHERE actor_user_id=$1 AND action='password_reset_completed'`, user.ID).Scan(&unlocked); err != nil {
		t.Fatal(err)
	}
	if unlocked != "true" {
		t.Fatalf("locked-account reset audit unlocked=%q, want true", unlocked)
	}

	loginService := newIntegrationLoginService(t, repository)
	result, err := loginService.Login(ctx, login.Input{Email: user.Email, Password: "correct horse battery"})
	if err != nil {
		t.Fatalf("Login after reset: %v", err)
	}
	if result.MfaToken == "" {
		t.Fatal("Login after reset returned no MFA challenge")
	}
}

// D13: a disabled account keeps its password change but stays disabled.
func TestRF015_D13_RestablecerNoReactivaCuentaDeshabilitada(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "d13-disabled@example.test", PasswordHash: "$argon2id$placeholder", DisplayName: "D13 Disabled"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'disabled' WHERE id = $1`, user.ID); err != nil {
		t.Fatalf("disable user: %v", err)
	}
	raw := []byte("d13-disabled-reset-token")
	hash := sha256.Sum256(raw)
	if _, err := pool.Exec(ctx, `INSERT INTO verification_tokens (user_id, token_hash, purpose, expires_at) VALUES ($1,$2,'password_reset',$3)`, user.ID, hash[:], time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	service := New(repository, integrationPublisher{}, integrationHasher{}, bytes.NewReader(make([]byte, 32)), time.Now)
	if err := service.Confirm(ctx, base64.RawURLEncoding.EncodeToString(raw), "correct horse battery"); err != nil {
		t.Fatalf("Confirm: %v", err)
	}
	var status, passwordHash string
	if err := pool.QueryRow(ctx, `SELECT status, password_hash FROM users WHERE id=$1`, user.ID).Scan(&status, &passwordHash); err != nil {
		t.Fatal(err)
	}
	if status != "disabled" {
		t.Fatalf("status=%q, want disabled to stay disabled", status)
	}
	if !strings.HasPrefix(passwordHash, "$argon2id$") || passwordHash == "$argon2id$placeholder" {
		t.Fatalf("password hash was not updated: %q", passwordHash)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE actor_user_id=$1 AND action='password_reset_completed'`, user.ID).Scan(&auditCount); err != nil {
		t.Fatal(err)
	}
	if auditCount != 1 {
		t.Fatalf("password_reset_completed audits=%d want 1", auditCount)
	}
	var unlocked string
	if err := pool.QueryRow(ctx, `SELECT metadata ->> 'unlocked' FROM audit_log WHERE actor_user_id=$1 AND action='password_reset_completed'`, user.ID).Scan(&unlocked); err != nil {
		t.Fatal(err)
	}
	if unlocked != "false" {
		t.Fatalf("disabled-account reset audit unlocked=%q, want false", unlocked)
	}
}

// D13: failures recorded before the completed reset must not count toward
// RF-017's lockout window, or a single wrong attempt right after a reset
// would immediately re-lock the account.
func TestRF015_D13_UnFalloTrasElRestablecimientoNoVuelveABloquear(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "d13-relock@example.test", PasswordHash: "$argon2id$placeholder", DisplayName: "D13 Relock"})
	if err != nil {
		t.Fatal(err)
	}
	// Simulate the lockout that already happened: five recent login_failed
	// audit rows (the default RF-017 threshold) plus the resulting lock.
	for range 5 {
		if _, err := pool.Exec(ctx, `INSERT INTO audit_log (actor_user_id, action, metadata) VALUES ($1, 'login_failed', '{}'::jsonb)`, user.ID); err != nil {
			t.Fatalf("seed login_failed audit: %v", err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'locked', locked_until = $2 WHERE id = $1`, user.ID, time.Now().Add(15*time.Minute)); err != nil {
		t.Fatalf("lock user: %v", err)
	}
	raw := []byte("d13-relock-reset-token")
	hash := sha256.Sum256(raw)
	if _, err := pool.Exec(ctx, `INSERT INTO verification_tokens (user_id, token_hash, purpose, expires_at) VALUES ($1,$2,'password_reset',$3)`, user.ID, hash[:], time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	service := New(repository, integrationPublisher{}, integrationHasher{}, bytes.NewReader(make([]byte, 32)), time.Now)
	if err := service.Confirm(ctx, base64.RawURLEncoding.EncodeToString(raw), "correct horse battery"); err != nil {
		t.Fatalf("Confirm: %v", err)
	}

	loginService := newIntegrationLoginService(t, repository)
	if _, err := loginService.Login(ctx, login.Input{Email: user.Email, Password: "wrong password"}); !errors.Is(err, login.ErrInvalidCredentials) {
		t.Fatalf("failed login after reset error = %v, want ErrInvalidCredentials", err)
	}

	var status string
	var lockedUntil *time.Time
	if err := pool.QueryRow(ctx, `SELECT status, locked_until FROM users WHERE id=$1`, user.ID).Scan(&status, &lockedUntil); err != nil {
		t.Fatal(err)
	}
	if status != "active" || lockedUntil != nil {
		t.Fatalf("status=%q locked_until=%v, want the account to stay unlocked after one failure", status, lockedUntil)
	}
}
