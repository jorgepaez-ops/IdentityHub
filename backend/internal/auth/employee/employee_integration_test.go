//go:build integration

package employee

import (
	"context"
	"crypto/rand"
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

func TestRF001_AltaPersisteHashArgon2idRolesYTokenDeInvitacion(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	ctx := context.Background()
	// granted_by REFERENCES users(id): the actor must be a real persisted
	// account, exactly like the authenticated admin the RequireRole
	// middleware already resolved before any real request reaches this
	// service (backend/internal/api/rbac.go).
	actorUser, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "admin-actor@example.test", PasswordHash: "$argon2id$actor-placeholder", DisplayName: "Admin actor"})
	if err != nil {
		t.Fatalf("create actor user: %v", err)
	}

	service := New(repository, integrationPublisher{}, passwordHasher{}, rand.Reader, time.Now)
	result, err := service.CreateEmployee(ctx, Input{
		Email: "employee-integration@example.test", DisplayName: "Employee integration",
		Roles: []string{"contabilidad.senior"}, ActorUserID: actorUser.ID,
	})
	if err != nil {
		t.Fatalf("CreateEmployee: %v", err)
	}

	var hash, status string
	if err := pool.QueryRow(ctx, `SELECT password_hash, status FROM users WHERE id = $1`, result.ID).Scan(&hash, &status); err != nil {
		t.Fatalf("read created user: %v", err)
	}
	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("password_hash = %q, want Argon2id PHC prefix (the admin never sets or learns this value)", hash)
	}
	if status != "pending_verification" {
		t.Errorf("status = %q, want pending_verification", status)
	}

	rows, err := pool.Query(ctx, `SELECT r.name FROM user_roles ur JOIN roles r ON r.id = ur.role_id WHERE ur.user_id = $1 ORDER BY r.name`, result.ID)
	if err != nil {
		t.Fatalf("query granted roles: %v", err)
	}
	defer rows.Close()
	var granted []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan role name: %v", err)
		}
		granted = append(granted, name)
	}
	if len(granted) != 2 || granted[0] != "contabilidad.senior" || granted[1] != "user" {
		t.Fatalf("granted roles = %v, want [contabilidad.senior user]", granted)
	}

	var purpose string
	var expiresAt time.Time
	if err := pool.QueryRow(ctx, `SELECT purpose, expires_at FROM verification_tokens WHERE user_id = $1`, result.ID).Scan(&purpose, &expiresAt); err != nil {
		t.Fatalf("read invitation token: %v", err)
	}
	if purpose != "invitation" {
		t.Fatalf("token purpose = %q, want invitation", purpose)
	}
	if time.Until(expiresAt) > 24*time.Hour || time.Until(expiresAt) < 23*time.Hour {
		t.Fatalf("invitation token expiry = %v, want ~24h from now", expiresAt)
	}
}

func TestRF001_CorreoDuplicadoDevuelveErrEmailExists(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	ctx := context.Background()
	actorUser, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "admin-actor-2@example.test", PasswordHash: "$argon2id$actor-placeholder", DisplayName: "Admin actor"})
	if err != nil {
		t.Fatalf("create actor user: %v", err)
	}
	service := New(repository, integrationPublisher{}, passwordHasher{}, rand.Reader, time.Now)
	input := Input{Email: "duplicate-employee@example.test", DisplayName: "First", Roles: []string{"user"}, ActorUserID: actorUser.ID}
	if _, err := service.CreateEmployee(ctx, input); err != nil {
		t.Fatalf("first CreateEmployee: %v", err)
	}
	input.DisplayName = "Second"
	if _, err := service.CreateEmployee(ctx, input); !errors.Is(err, ErrEmailExists) {
		t.Fatalf("second CreateEmployee() error = %v, want ErrEmailExists", err)
	}
}
