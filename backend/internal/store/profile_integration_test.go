//go:build integration

package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRF008_ActualizaNombreDeVisualizacionPersisteElCambio(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}

	created, err := repository.CreateUser(ctx, store.CreateUserParams{
		Email:        "profile-owner@example.test",
		PasswordHash: "$argon2id$fixed-test-value",
		DisplayName:  "Old Name",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	updated, err := repository.UpdateDisplayName(ctx, created.ID, "New Name")
	if err != nil {
		t.Fatalf("UpdateDisplayName: %v", err)
	}
	if updated.DisplayName != "New Name" {
		t.Errorf("UpdateDisplayName returned DisplayName = %q, want %q", updated.DisplayName, "New Name")
	}

	reloaded, err := repository.GetUserByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUserByID after update: %v", err)
	}
	if reloaded.DisplayName != "New Name" {
		t.Errorf("persisted DisplayName = %q, want %q", reloaded.DisplayName, "New Name")
	}
}

func TestRF008_ActualizaNombreDeVisualizacionDevuelveNoRowsParaIDInexistente(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}

	if _, err := repository.UpdateDisplayName(ctx, uuid.New(), "Ghost"); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("UpdateDisplayName(unknown id) error = %v, want pgx.ErrNoRows", err)
	}
}
