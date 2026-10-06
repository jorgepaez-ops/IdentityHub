//go:build integration

package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/rolegrid"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRF021_StoreListaAplicacionesConPermisosRolesYConteos(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	user := createAdminTestUser(t, ctx, repository, "grid-list@example.test", "Grid List", "")
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = 'contabilidad.analista'`, user.ID); err != nil {
		t.Fatal(err)
	}

	applications, err := repository.ListApplications(ctx)
	if err != nil {
		t.Fatalf("ListApplications() error = %v", err)
	}
	if len(applications) != 1 || applications[0].ClientID != "contabilidad" || applications[0].Name != "Contabilidad" || len(applications[0].Permissions) != 5 {
		t.Fatalf("applications = %+v", applications)
	}
	roles := map[string]rolegrid.Role{}
	for _, role := range applications[0].Roles {
		roles[role.Name] = role
	}
	if !reflect.DeepEqual(roles["contabilidad.analista"].PermissionKeys, []string{"movimientos.registrar", "reportes.ver"}) || roles["contabilidad.analista"].AssignedCount != 1 || roles["contabilidad.senior"].AssignedCount != 0 || len(roles["contabilidad.senior"].PermissionKeys) != 5 {
		t.Fatalf("roles = %+v", roles)
	}
	if _, err := repository.ListRoles(ctx, uuid.New()); !errors.Is(err, rolegrid.ErrApplicationNotFound) {
		t.Fatalf("ListRoles(unknown) error = %v, want ErrApplicationNotFound", err)
	}
	listed, err := repository.ListRoles(ctx, applications[0].ID)
	if err != nil || len(listed) != 2 {
		t.Fatalf("ListRoles() = %v, %v", listed, err)
	}
	for _, table := range []string{"permissions", "role_permissions", "roles", "applications", "user_roles"} {
		var ok bool
		if err := pool.QueryRow(ctx, `SELECT has_table_privilege('identity_app', $1, 'SELECT')`, table).Scan(&ok); err != nil || !ok {
			t.Fatalf("identity_app lacks SELECT on %s (err=%v)", table, err)
		}
	}
	for _, table := range []string{"roles", "role_permissions"} {
		for _, privilege := range []string{"INSERT", "UPDATE", "DELETE"} {
			if table == "role_permissions" && privilege == "UPDATE" {
				continue
			}
			var ok bool
			if err := pool.QueryRow(ctx, `SELECT has_table_privilege('identity_app', $1, $2)`, table, privilege).Scan(&ok); err != nil || !ok {
				t.Fatalf("identity_app lacks %s on %s (err=%v)", privilege, table, err)
			}
		}
	}
}

func TestRF021_ServicioConStoreCreaActualizaYEliminaConAuditoria(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	service := rolegrid.New(repository)
	actor := createAdminTestUser(t, ctx, repository, "grid-actor@example.test", "Grid Actor", "")
	applications, err := repository.ListApplications(ctx)
	if err != nil {
		t.Fatal(err)
	}
	applicationID := applications[0].ID
	ip := netip.MustParseAddr("203.0.113.20")

	role, err := service.Create(ctx, rolegrid.CreateInput{ActorUserID: actor.ID, ApplicationID: applicationID, Name: "contabilidad.auditor", Description: "Solo lectura", PermissionKeys: []string{"reportes.ver", "movimientos.ver_todos"}, IP: ip, UserAgent: "itest"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !reflect.DeepEqual(role.PermissionKeys, []string{"movimientos.ver_todos", "reportes.ver"}) {
		t.Fatalf("Create() keys = %v", role.PermissionKeys)
	}
	if _, err := service.Create(ctx, rolegrid.CreateInput{ActorUserID: actor.ID, ApplicationID: applicationID, Name: "contabilidad.auditor", PermissionKeys: nil}); !errors.Is(err, rolegrid.ErrDuplicateRole) {
		t.Fatalf("duplicate Create() error = %v, want ErrDuplicateRole", err)
	}
	if _, err := service.Create(ctx, rolegrid.CreateInput{ActorUserID: actor.ID, ApplicationID: applicationID, Name: "contabilidad.otro", PermissionKeys: []string{"usuarios.borrar"}}); !errors.Is(err, rolegrid.ErrInvalidPermission) {
		t.Fatalf("unknown permission error = %v", err)
	}
	var leaked int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM roles WHERE name = 'contabilidad.otro'`).Scan(&leaked); err != nil || leaked != 0 {
		t.Fatalf("rejected role persisted: count=%d err=%v", leaked, err)
	}

	keys := []string{"cierre.ejecutar"}
	description := "Con cierre"
	updated, err := service.Update(ctx, rolegrid.UpdateInput{ActorUserID: actor.ID, ApplicationID: applicationID, RoleID: role.ID, Description: &description, PermissionKeys: &keys, IP: ip, UserAgent: "itest"})
	if err != nil || updated.Description != "Con cierre" || !reflect.DeepEqual(updated.PermissionKeys, keys) {
		t.Fatalf("Update() = %+v, %v", updated, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, actor.ID, role.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, rolegrid.DeleteInput{ActorUserID: actor.ID, ApplicationID: applicationID, RoleID: role.ID}); !errors.Is(err, rolegrid.ErrSelfRoleDelete) {
		t.Fatalf("Delete(held) error = %v, want ErrSelfRoleDelete", err)
	}
	if _, err := service.Update(ctx, rolegrid.UpdateInput{ActorUserID: actor.ID, ApplicationID: applicationID, RoleID: role.ID, PermissionKeys: &[]string{"reportes.ver"}}); !errors.Is(err, rolegrid.ErrSelfPermissionChange) {
		t.Fatalf("Update(held) error = %v, want ErrSelfPermissionChange", err)
	}
	other := createAdminTestUser(t, ctx, repository, "grid-other@example.test", "Grid Other", "")
	if err := service.Delete(ctx, rolegrid.DeleteInput{ActorUserID: other.ID, ApplicationID: applicationID, RoleID: role.ID}); !errors.Is(err, rolegrid.ErrRoleAssigned) {
		t.Fatalf("Delete(assigned) error = %v, want ErrRoleAssigned", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM user_roles WHERE role_id = $1`, role.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, rolegrid.DeleteInput{ActorUserID: other.ID, ApplicationID: applicationID, RoleID: role.ID, IP: ip, UserAgent: "itest"}); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if err := service.Delete(ctx, rolegrid.DeleteInput{ActorUserID: other.ID, ApplicationID: applicationID, RoleID: role.ID}); !errors.Is(err, rolegrid.ErrRoleNotFound) {
		t.Fatalf("Delete(gone) error = %v, want ErrRoleNotFound", err)
	}

	rows, err := pool.Query(ctx, `SELECT action, actor_user_id, resource_type, resource_id, host(ip), user_agent, metadata FROM audit_log WHERE action LIKE 'role\_%' AND action <> 'role_changed' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var actions []string
	for rows.Next() {
		var action, resourceType, resourceID, address, userAgent string
		var actorID uuid.UUID
		var metadata []byte
		if err := rows.Scan(&action, &actorID, &resourceType, &resourceID, &address, &userAgent, &metadata); err != nil {
			t.Fatal(err)
		}
		var body struct {
			RoleName       string   `json:"roleName"`
			PermissionKeys []string `json:"permissionKeys"`
		}
		if err := json.Unmarshal(metadata, &body); err != nil {
			t.Fatal(err)
		}
		if resourceType != "application_role" || resourceID != role.ID.String() || address != "203.0.113.20" || userAgent != "itest" || body.RoleName != "contabilidad.auditor" {
			t.Fatalf("audit row %s: type=%s id=%s ip=%s ua=%s meta=%s", action, resourceType, resourceID, address, userAgent, metadata)
		}
		if action == "role_updated" && !reflect.DeepEqual(body.PermissionKeys, keys) {
			t.Fatalf("role_updated keys = %v", body.PermissionKeys)
		}
		actions = append(actions, action)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"role_created", "role_updated", "role_deleted"}; !reflect.DeepEqual(actions, want) {
		t.Fatalf("audit actions = %v, want %v", actions, want)
	}
}

func TestRF021_StoreMapeaElTriggerYLaRestriccionDeAsignaciones(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	var adminRoleID, analystRoleID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM roles WHERE name = 'admin'`).Scan(&adminRoleID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM roles WHERE name = 'contabilidad.analista'`).Scan(&analystRoleID); err != nil {
		t.Fatal(err)
	}
	user := createAdminTestUser(t, ctx, repository, "grid-trigger@example.test", "Grid Trigger", "")
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, user.ID, analystRoleID); err != nil {
		t.Fatal(err)
	}
	// The service refuses these earlier; the adapter must still translate the database refusals.
	err = repository.WithinRoleGridTransaction(ctx, func(w rolegrid.Writer) error { return w.DeleteApplicationRole(ctx, adminRoleID) })
	if !errors.Is(err, rolegrid.ErrSystemRole) {
		t.Fatalf("delete system role error = %v, want ErrSystemRole", err)
	}
	err = repository.WithinRoleGridTransaction(ctx, func(w rolegrid.Writer) error { return w.DeleteApplicationRole(ctx, analystRoleID) })
	if !errors.Is(err, rolegrid.ErrRoleAssigned) {
		t.Fatalf("delete assigned role error = %v, want ErrRoleAssigned", err)
	}
	err = repository.WithinRoleGridTransaction(ctx, func(w rolegrid.Writer) error {
		return w.UpdateApplicationRoleDescription(ctx, adminRoleID, "x")
	})
	if !errors.Is(err, rolegrid.ErrSystemRole) {
		t.Fatalf("update system role error = %v, want ErrSystemRole", err)
	}
	// A role of another application is not addressable through this application (404).
	var foreignID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO applications (client_id, redirect_uri, allowed_origin, name) VALUES ('otra', 'http://otra.test/cb', 'http://otra.test', 'Otra') RETURNING id`).Scan(&foreignID); err != nil {
		t.Fatal(err)
	}
	err = repository.WithinRoleGridTransaction(ctx, func(w rolegrid.Writer) error {
		_, err := w.GetApplicationRoleForUpdate(ctx, foreignID, analystRoleID)
		return err
	})
	if !errors.Is(err, rolegrid.ErrRoleNotFound) {
		t.Fatalf("cross-application lookup error = %v, want ErrRoleNotFound", err)
	}
}

func TestRF021_StoreBloqueaLaFilaDelRolYMapeaRestriccionesPorNombre(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	user := createAdminTestUser(t, ctx, repository, "grid-lock@example.test", "Grid Lock", "")
	var roleID, appID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT r.id, r.application_id FROM roles r WHERE r.name = 'contabilidad.senior'`).Scan(&roleID, &appID); err != nil {
		t.Fatal(err)
	}

	// While the role row is locked, assigning the role (user_roles FK) must wait for the transaction.
	assigned := make(chan error, 1)
	err = repository.WithinRoleGridTransaction(ctx, func(w rolegrid.Writer) error {
		if _, err := w.GetApplicationRoleForUpdate(ctx, appID, roleID); err != nil {
			return err
		}
		go func() {
			_, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)`, user.ID, roleID)
			assigned <- err
		}()
		select {
		case err := <-assigned:
			t.Errorf("assignment finished while the role row was locked (err=%v)", err)
		case <-time.After(500 * time.Millisecond):
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-assigned:
		if err != nil {
			t.Fatalf("assignment after the lock was released: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("assignment never completed after the lock was released")
	}

	// A missing application is ErrApplicationNotFound and a duplicate name ErrDuplicateRole.
	err = repository.WithinRoleGridTransaction(ctx, func(w rolegrid.Writer) error {
		_, err := w.CreateApplicationRole(ctx, uuid.New(), "ghost.rol", "")
		return err
	})
	if !errors.Is(err, rolegrid.ErrApplicationNotFound) {
		t.Fatalf("create in missing application error = %v, want ErrApplicationNotFound", err)
	}
	err = repository.WithinRoleGridTransaction(ctx, func(w rolegrid.Writer) error {
		_, err := w.CreateApplicationRole(ctx, appID, "contabilidad.senior", "")
		return err
	})
	if !errors.Is(err, rolegrid.ErrDuplicateRole) {
		t.Fatalf("duplicate create error = %v, want ErrDuplicateRole", err)
	}
}
