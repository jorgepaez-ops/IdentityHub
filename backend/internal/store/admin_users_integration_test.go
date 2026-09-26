//go:build integration

package store_test

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/admin"
	"github.com/jorgepaez/identity-hub/internal/auth/auditlog"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func createAdminTestUser(t *testing.T, ctx context.Context, repository *store.Store, email, displayName, status string) store.User {
	t.Helper()
	user, err := repository.CreateUser(ctx, store.CreateUserParams{
		Email:        email,
		PasswordHash: "$argon2id$fixed-test-value",
		DisplayName:  displayName,
	})
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", email, err)
	}
	if status != "" {
		if _, err := repository.Pool().Exec(ctx, `UPDATE users SET status = $1 WHERE id = $2`, status, user.ID); err != nil {
			t.Fatalf("set status for %s: %v", email, err)
		}
	}
	return user
}

func TestRF010_ListaUsuariosFiltraPorEstadoBusquedaYCursor(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}

	active := createAdminTestUser(t, ctx, repository, "list-admin-one@example.test", "Admin One", "active")
	locked := createAdminTestUser(t, ctx, repository, "list-admin-two@example.test", "Admin Two", "locked")
	other := createAdminTestUser(t, ctx, repository, "list-other@example.test", "Someone Else", "")

	all, err := repository.ListUsers(ctx, admin.ListInput{Limit: 10})
	if err != nil {
		t.Fatalf("ListUsers(all): %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("ListUsers(all) returned %d users, want 3", len(all))
	}
	for _, user := range all {
		if len(user.Roles) != 0 {
			t.Errorf("ListUsers(all) user %s has roles %v, want none granted yet", user.Email, user.Roles)
		}
	}

	byQuery, err := repository.ListUsers(ctx, admin.ListInput{Query: "Admin", Limit: 10})
	if err != nil {
		t.Fatalf("ListUsers(query=Admin): %v", err)
	}
	if len(byQuery) != 2 {
		t.Fatalf("ListUsers(query=Admin) returned %d users, want 2 (%v)", len(byQuery), byQuery)
	}

	activeStatus := admin.StatusActive
	byStatus, err := repository.ListUsers(ctx, admin.ListInput{Status: &activeStatus, Limit: 10})
	if err != nil {
		t.Fatalf("ListUsers(status=active): %v", err)
	}
	if len(byStatus) != 1 || byStatus[0].ID != active.ID {
		t.Fatalf("ListUsers(status=active) = %v, want only %s", byStatus, active.ID)
	}

	// Keyset pagination: sort the known IDs and ask for the page strictly after
	// the first one, exercising the "$3::uuid IS NULL OR id > $3::uuid" branch
	// that a single, cursor-less page never reaches.
	ids := []uuid.UUID{active.ID, locked.ID, other.ID}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	afterFirst, err := repository.ListUsers(ctx, admin.ListInput{Cursor: &ids[0], Limit: 10})
	if err != nil {
		t.Fatalf("ListUsers(cursor): %v", err)
	}
	if len(afterFirst) != 2 {
		t.Fatalf("ListUsers(cursor) returned %d users, want 2", len(afterFirst))
	}
	for _, user := range afterFirst {
		if user.ID == ids[0] {
			t.Errorf("ListUsers(cursor) still returned the cursor's own user %s", user.ID)
		}
	}
}

func TestRNF011_ListaUsuariosConEntradaTipoInyeccionNoDevuelveFilas(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	createAdminTestUser(t, ctx, repository, "injection-target@example.test", "Injection Target", "")

	rows, err := repository.ListUsers(ctx, admin.ListInput{Query: "' OR '1'='1' --", Limit: 10})
	if err != nil {
		t.Fatalf("ListUsers(injection query) returned an error instead of zero rows: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("ListUsers(injection query) = %d rows, want 0", len(rows))
	}
}

func TestRF010_GetUserDevuelveErrUserNotFoundParaIDInexistente(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}

	if _, err := repository.GetUser(ctx, uuid.New()); !errors.Is(err, admin.ErrUserNotFound) {
		t.Fatalf("GetUser(unknown id) error = %v, want admin.ErrUserNotFound", err)
	}

	created := createAdminTestUser(t, ctx, repository, "get-admin-user@example.test", "Gettable User", "")
	found, err := repository.GetUser(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUser(known id): %v", err)
	}
	if found.Email != created.Email || len(found.Roles) != 0 {
		t.Fatalf("GetUser(known id) = %+v, want email %q and no roles", found, created.Email)
	}
}

func TestRF010_ActualizaEstadoYRolesPersisteEnUnaTransaccion(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}

	actor := createAdminTestUser(t, ctx, repository, "management-admin@example.test", "Management Admin", "active")
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id, granted_by) SELECT $1, id, $1 FROM roles WHERE name = 'admin'`, actor.ID); err != nil {
		t.Fatalf("grant admin to actor: %v", err)
	}
	target := createAdminTestUser(t, ctx, repository, "management-target@example.test", "Management Target", "active")

	var lockedActiveAdmins int64
	var rolesDuringTransaction []string
	err = repository.WithinUserManagementTransaction(ctx, func(writer admin.Writer) error {
		var lockErr error
		lockedActiveAdmins, lockErr = writer.LockActiveAdmins(ctx)
		if lockErr != nil {
			return lockErr
		}
		if _, err := writer.GetUserForUpdate(ctx, target.ID); err != nil {
			return err
		}
		if _, err := writer.UpdateUser(ctx, target.ID, admin.StatusDisabled); err != nil {
			return err
		}
		if err := writer.InsertAuditEvent(ctx, admin.AuditEvent{ActorUserID: &actor.ID, Action: "user_disabled", ResourceType: "user", ResourceID: target.ID.String()}); err != nil {
			return err
		}
		if err := writer.ReplaceRoles(ctx, target.ID, []string{"admin"}, actor.ID); err != nil {
			return err
		}
		if err := writer.InsertAuditEvent(ctx, admin.AuditEvent{ActorUserID: &actor.ID, Action: "role_changed", ResourceType: "user", ResourceID: target.ID.String()}); err != nil {
			return err
		}
		var listErr error
		rolesDuringTransaction, listErr = writer.ListRolesForUser(ctx, target.ID)
		return listErr
	})
	if err != nil {
		t.Fatalf("WithinUserManagementTransaction: %v", err)
	}
	if lockedActiveAdmins < 1 {
		t.Errorf("LockActiveAdmins locked %d rows, want at least 1 (the actor)", lockedActiveAdmins)
	}
	if len(rolesDuringTransaction) != 1 || rolesDuringTransaction[0] != "admin" {
		t.Errorf("roles read inside the transaction = %v, want [admin]", rolesDuringTransaction)
	}

	persisted, err := repository.GetUser(ctx, target.ID)
	if err != nil {
		t.Fatalf("GetUser after commit: %v", err)
	}
	if persisted.Status != admin.StatusDisabled {
		t.Errorf("persisted status = %q, want disabled", persisted.Status)
	}
	if len(persisted.Roles) != 1 || persisted.Roles[0] != "admin" {
		t.Errorf("persisted roles = %v, want [admin]", persisted.Roles)
	}

	events, err := repository.ListAuditLog(ctx, auditlog.ListInput{ActorID: &actor.ID, Limit: 10})
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}
	actions := make(map[string]bool)
	for _, event := range events {
		actions[event.Action] = true
		if event.IP != nil {
			t.Errorf("admin audit event %q carried an IP %q, want none (adminWriter never sets it)", event.Action, *event.IP)
		}
	}
	if !actions["user_disabled"] || !actions["role_changed"] {
		t.Fatalf("audit events = %v, want user_disabled and role_changed", actions)
	}
}

func TestRF010_TransaccionDeAdministracionSeRevierteSiElEscritorFalla(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	target := createAdminTestUser(t, ctx, repository, "rollback-target@example.test", "Rollback Target", "active")

	txErr := repository.WithinUserManagementTransaction(ctx, func(writer admin.Writer) error {
		if err := writer.ReplaceRoles(ctx, target.ID, []string{"admin"}, target.ID); err != nil {
			return err
		}
		if err := writer.InsertAuditEvent(ctx, admin.AuditEvent{Action: "role_changed", ResourceType: "user", ResourceID: target.ID.String()}); err != nil {
			return err
		}
		// GetUserForUpdate on an unrelated, nonexistent id fails after the writes
		// above, proving the whole transaction (including the role grant already
		// issued in this same callback) rolls back instead of partially applying.
		_, err := writer.GetUserForUpdate(ctx, uuid.New())
		return err
	})
	if !errors.Is(txErr, admin.ErrUserNotFound) {
		t.Fatalf("WithinUserManagementTransaction error = %v, want admin.ErrUserNotFound", txErr)
	}

	roles, err := repository.ListRolesForUser(ctx, target.ID)
	if err != nil {
		t.Fatalf("ListRolesForUser after rollback: %v", err)
	}
	if len(roles) != 0 {
		t.Fatalf("roles after rollback = %v, want none (ReplaceRoles must have been rolled back)", roles)
	}

	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action = 'role_changed' AND resource_id = $1`, target.ID.String()).Scan(&auditCount); err != nil {
		t.Fatalf("count audit rows after rollback: %v", err)
	}
	if auditCount != 0 {
		t.Fatalf("audit rows after rollback = %d, want 0 (InsertAuditEvent must have been rolled back too)", auditCount)
	}
}
