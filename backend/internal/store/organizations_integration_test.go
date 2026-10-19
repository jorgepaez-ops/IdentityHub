//go:build integration

package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jorgepaez/identity-hub/internal/auth/admin"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

// createOrganizationUser creates an account in the given organization.
func createOrganizationUser(t *testing.T, ctx context.Context, repository *store.Store, organizationID uuid.UUID, email string) store.User {
	t.Helper()
	user, err := repository.CreateUser(ctx, store.CreateUserParams{
		Email:          email,
		PasswordHash:   "$argon2id$fixed-test-value",
		DisplayName:    email,
		OrganizationID: organizationID,
	})
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", email, err)
	}
	return user
}

func requireSQLState(t *testing.T, err error, want string) {
	t.Helper()
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != want {
		t.Fatalf("error = %v, want SQLSTATE %s", err, want)
	}
}

func assignRoleSQL(ctx context.Context, pool *pgxpool.Pool, userID uuid.UUID, roleName string) error {
	_, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = $2`, userID, roleName)
	return err
}

func TestRF022_MigracionSiembraEmpresasAplicacionesYRoles(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}

	organizations, err := repository.ListOrganizations(ctx)
	if err != nil {
		t.Fatalf("ListOrganizations() error = %v", err)
	}
	got := map[string]uuid.UUID{}
	for _, organization := range organizations {
		got[organization.Slug] = organization.ID
	}
	want := map[string]uuid.UUID{
		"hub":             store.HubOrganizationID,
		"contabilidad-sa": store.ContabilidadOrganizationID,
		"empresa-b":       store.EmpresaBOrganizationID,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("organizations = %v, want %v", got, want)
	}

	owners := map[string]uuid.UUID{}
	rows, err := pool.Query(ctx, `SELECT client_id, organization_id FROM applications`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var clientID string
		var organizationID uuid.UUID
		if err := rows.Scan(&clientID, &organizationID); err != nil {
			t.Fatal(err)
		}
		owners[clientID] = organizationID
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if owners["contabilidad"] != store.ContabilidadOrganizationID || owners["inventario"] != store.EmpresaBOrganizationID || len(owners) != 2 {
		t.Fatalf("application owners = %v", owners)
	}

	var redirectURI, allowedOrigin string
	if err := pool.QueryRow(ctx, `SELECT redirect_uri, allowed_origin FROM applications WHERE client_id = 'inventario'`).Scan(&redirectURI, &allowedOrigin); err != nil {
		t.Fatal(err)
	}
	if redirectURI != "http://inventario.localhost:8080/oauth/callback" || allowedOrigin != "http://inventario.localhost:8080" {
		t.Fatalf("inventario client = %q, %q", redirectURI, allowedOrigin)
	}
	assertSeededRolePermissions(t, ctx, pool, "inventario.consulta", []string{"productos.ver"})
	assertSeededRolePermissions(t, ctx, pool, "inventario.bodeguero", []string{"movimientos.registrar", "productos.editar", "productos.ver"})
	var systemRoles int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM roles WHERE name LIKE 'inventario.%' AND NOT system AND application_id IS NOT NULL`).Scan(&systemRoles); err != nil || systemRoles != 2 {
		t.Fatalf("inventario application roles = %d, err = %v, want 2", systemRoles, err)
	}
}

func TestRF022_StoreRechazaRolDeOtraEmpresa(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	actor := createOrganizationUser(t, ctx, repository, store.HubOrganizationID, "rf022-actor@example.test")
	contabilidadUser := createOrganizationUser(t, ctx, repository, store.ContabilidadOrganizationID, "rf022-conta@example.test")
	empresaBUser := createOrganizationUser(t, ctx, repository, store.EmpresaBOrganizationID, "rf022-b@example.test")

	// Admin path (ReplaceRoles): a Contabilidad account cannot receive an Inventario role.
	err = repository.WithinUserManagementTransaction(ctx, func(w admin.Writer) error {
		return w.ReplaceRoles(ctx, contabilidadUser.ID, []string{"user", "inventario.consulta"}, actor.ID)
	})
	if !errors.Is(err, store.ErrCrossOrganizationRole) {
		t.Fatalf("ReplaceRoles(cross-company) error = %v, want ErrCrossOrganizationRole", err)
	}
	// Employee creation path (AddUserRole) maps the same typed error.
	err = repository.WithinEmployeeCreationTransaction(ctx, func(w store.EmployeeCreationWriter) error {
		user, err := w.CreateUser(ctx, store.CreateUserParams{Email: "rf022-new@example.test", PasswordHash: "$argon2id$fixed-test-value", DisplayName: "New", OrganizationID: store.ContabilidadOrganizationID})
		if err != nil {
			return err
		}
		return w.AddUserRole(ctx, user.ID, "inventario.bodeguero", actor.ID)
	})
	if !errors.Is(err, store.ErrCrossOrganizationRole) {
		t.Fatalf("AddUserRole(cross-company) error = %v, want ErrCrossOrganizationRole", err)
	}

	// The rejected transaction left nothing behind and the same-company assignments succeed.
	roles, err := repository.ListRolesForUser(ctx, contabilidadUser.ID)
	if err != nil || len(roles) != 0 {
		t.Fatalf("roles after rejection = %v, err = %v, want none", roles, err)
	}
	err = repository.WithinUserManagementTransaction(ctx, func(w admin.Writer) error {
		if err := w.ReplaceRoles(ctx, empresaBUser.ID, []string{"user", "inventario.bodeguero"}, actor.ID); err != nil {
			return err
		}
		return w.ReplaceRoles(ctx, contabilidadUser.ID, []string{"user", "contabilidad.analista"}, actor.ID)
	})
	if err != nil {
		t.Fatalf("same-company ReplaceRoles() error = %v", err)
	}
	// And the reverse direction is rejected too.
	err = repository.WithinUserManagementTransaction(ctx, func(w admin.Writer) error {
		return w.ReplaceRoles(ctx, empresaBUser.ID, []string{"user", "contabilidad.senior"}, actor.ID)
	})
	if !errors.Is(err, store.ErrCrossOrganizationRole) {
		t.Fatalf("ReplaceRoles(Empresa B -> Contabilidad role) error = %v, want ErrCrossOrganizationRole", err)
	}
}

func TestRF022_TriggerImpideAsignarRolDeOtraEmpresaConSQLCrudo(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	contabilidadUser := createOrganizationUser(t, ctx, repository, store.ContabilidadOrganizationID, "rf022-raw-conta@example.test")

	requireSQLState(t, assignRoleSQL(ctx, pool, contabilidadUser.ID, "inventario.consulta"), "IH001")
	// Moving an existing assignment to a foreign role through UPDATE is refused as well.
	if err := assignRoleSQL(ctx, pool, contabilidadUser.ID, "contabilidad.analista"); err != nil {
		t.Fatalf("same-company assignment: %v", err)
	}
	_, err = pool.Exec(ctx, `UPDATE user_roles SET role_id = (SELECT id FROM roles WHERE name = 'inventario.consulta') WHERE user_id = $1`, contabilidadUser.ID)
	requireSQLState(t, err, "IH001")
}

func TestRF022_RolesDeDirectorioNoSeRestringenPorEmpresa(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	for name, organizationID := range map[string]uuid.UUID{
		"hub":             store.HubOrganizationID,
		"contabilidad-sa": store.ContabilidadOrganizationID,
		"empresa-b":       store.EmpresaBOrganizationID,
	} {
		user := createOrganizationUser(t, ctx, repository, organizationID, "rf022-dir-"+name+"@example.test")
		for _, role := range []string{"user", "admin"} {
			if err := assignRoleSQL(ctx, pool, user.ID, role); err != nil {
				t.Fatalf("directory role %s for %s: %v", role, name, err)
			}
		}
	}
}

func TestRF022_CambiarEmpresaConRolesDeLaAnteriorSeRechaza(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	user := createOrganizationUser(t, ctx, repository, store.ContabilidadOrganizationID, "rf022-move@example.test")
	for _, role := range []string{"user", "contabilidad.analista"} {
		if err := assignRoleSQL(ctx, pool, user.ID, role); err != nil {
			t.Fatal(err)
		}
	}
	_, err = pool.Exec(ctx, `UPDATE users SET organization_id = $2 WHERE id = $1`, user.ID, store.EmpresaBOrganizationID)
	requireSQLState(t, err, "IH001")
	organization, err := repository.GetUserOrganization(ctx, user.ID)
	if err != nil || organization.ID != store.ContabilidadOrganizationID {
		t.Fatalf("organization after rejected move = %+v, err = %v", organization, err)
	}

	// Once the application roles are gone (directory roles do not count) the move is allowed.
	if _, err := pool.Exec(ctx, `DELETE FROM user_roles WHERE user_id = $1 AND role_id = (SELECT id FROM roles WHERE name = 'contabilidad.analista')`, user.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET organization_id = $2 WHERE id = $1`, user.ID, store.EmpresaBOrganizationID); err != nil {
		t.Fatalf("move without application roles: %v", err)
	}
	organization, err = repository.GetUserOrganization(ctx, user.ID)
	if err != nil || organization.ID != store.EmpresaBOrganizationID || organization.Slug != "empresa-b" {
		t.Fatalf("organization after move = %+v, err = %v", organization, err)
	}
}

func TestRF022_ListadosPorEmpresaDevuelvenSoloSusFilas(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	conta1 := createOrganizationUser(t, ctx, repository, store.ContabilidadOrganizationID, "rf022-list-c1@example.test")
	conta2 := createOrganizationUser(t, ctx, repository, store.ContabilidadOrganizationID, "rf022-list-c2@example.test")
	b1 := createOrganizationUser(t, ctx, repository, store.EmpresaBOrganizationID, "rf022-list-b1@example.test")
	hub := createOrganizationUser(t, ctx, repository, store.HubOrganizationID, "rf022-list-hub@example.test")

	ids := func(users []admin.User) map[uuid.UUID]bool {
		result := map[uuid.UUID]bool{}
		for _, user := range users {
			result[user.ID] = true
		}
		return result
	}
	contaUsers, err := repository.ListOrganizationUsers(ctx, store.ContabilidadOrganizationID, admin.ListInput{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(contaUsers); !reflect.DeepEqual(got, map[uuid.UUID]bool{conta1.ID: true, conta2.ID: true}) {
		t.Fatalf("Contabilidad users = %v", got)
	}
	bUsers, err := repository.ListOrganizationUsers(ctx, store.EmpresaBOrganizationID, admin.ListInput{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(bUsers); !reflect.DeepEqual(got, map[uuid.UUID]bool{b1.ID: true}) {
		t.Fatalf("Empresa B users = %v", got)
	}
	hubUsers, err := repository.ListOrganizationUsers(ctx, store.HubOrganizationID, admin.ListInput{Limit: 50})
	if err != nil || !reflect.DeepEqual(ids(hubUsers), map[uuid.UUID]bool{hub.ID: true}) {
		t.Fatalf("hub users = %v, err = %v", hubUsers, err)
	}
	// Search and paging keep working inside one organization.
	page, err := repository.ListOrganizationUsers(ctx, store.ContabilidadOrganizationID, admin.ListInput{Limit: 1})
	if err != nil || len(page) != 1 {
		t.Fatalf("first page = %v, err = %v", page, err)
	}
	next, err := repository.ListOrganizationUsers(ctx, store.ContabilidadOrganizationID, admin.ListInput{Limit: 1, Cursor: &page[0].ID})
	if err != nil || len(next) != 1 || next[0].ID == page[0].ID {
		t.Fatalf("second page = %v, err = %v", next, err)
	}
	found, err := repository.ListOrganizationUsers(ctx, store.EmpresaBOrganizationID, admin.ListInput{Limit: 50, Query: "list-c1"})
	if err != nil || len(found) != 0 {
		t.Fatalf("cross-company search = %v, err = %v, want none", found, err)
	}

	applications, err := repository.ListOrganizationApplications(ctx, store.EmpresaBOrganizationID)
	if err != nil || len(applications) != 1 || applications[0].ClientID != "inventario" || applications[0].OrganizationID != store.EmpresaBOrganizationID {
		t.Fatalf("Empresa B applications = %+v, err = %v", applications, err)
	}
	applications, err = repository.ListOrganizationApplications(ctx, store.ContabilidadOrganizationID)
	if err != nil || len(applications) != 1 || applications[0].ClientID != "contabilidad" {
		t.Fatalf("Contabilidad applications = %+v, err = %v", applications, err)
	}
	applications, err = repository.ListOrganizationApplications(ctx, store.HubOrganizationID)
	if err != nil || len(applications) != 0 {
		t.Fatalf("hub applications = %+v, err = %v, want none", applications, err)
	}
}

func TestRF022_CrearUsuarioExigeEmpresa(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repository.CreateUser(ctx, store.CreateUserParams{Email: "rf022-noorg@example.test", PasswordHash: "$argon2id$fixed-test-value", DisplayName: "No org"})
	if !errors.Is(err, store.ErrOrganizationRequired) {
		t.Fatalf("CreateUser(no organization) error = %v, want ErrOrganizationRequired", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO users (email, password_hash, display_name) VALUES ('rf022-raw@example.test', '$argon2id$x', 'Raw')`)
	requireSQLState(t, err, "23502")
}
