//go:build integration

package store_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jorgepaez/identity-hub/internal/auth/admin"
	"github.com/jorgepaez/identity-hub/internal/auth/oauth"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRF021_MigracionSiembraPermisosYProtegeRolesDelSistema(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)

	var permissionCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM permissions p JOIN applications a ON a.id = p.application_id WHERE a.client_id = 'contabilidad'`).Scan(&permissionCount); err != nil {
		t.Fatalf("count seeded permissions: %v", err)
	}
	if permissionCount != 5 {
		t.Fatalf("seeded permissions=%d, want 5", permissionCount)
	}
	assertSeededRolePermissions(t, ctx, pool, "contabilidad.senior", []string{"cierre.ejecutar", "movimientos.aprobar", "movimientos.registrar", "movimientos.ver_todos", "reportes.ver"})
	assertSeededRolePermissions(t, ctx, pool, "contabilidad.analista", []string{"movimientos.registrar", "reportes.ver"})
	for _, name := range []string{"admin", "user"} {
		var system bool
		if err := pool.QueryRow(ctx, `SELECT system FROM roles WHERE name = $1`, name).Scan(&system); err != nil || !system {
			t.Fatalf("system role %q system=%t err=%v", name, system, err)
		}
	}
	for _, name := range []string{"contabilidad.senior", "contabilidad.analista"} {
		var clientID string
		if err := pool.QueryRow(ctx, `SELECT a.client_id FROM roles r JOIN applications a ON a.id = r.application_id WHERE r.name = $1`, name).Scan(&clientID); err != nil || clientID != "contabilidad" {
			t.Fatalf("application role %q client=%q err=%v", name, clientID, err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE roles SET description = 'mutated' WHERE name = 'admin'`); err == nil {
		t.Fatal("UPDATE system admin role succeeded")
	}
	if _, err := pool.Exec(ctx, `DELETE FROM roles WHERE name = 'admin'`); err == nil {
		t.Fatal("DELETE system admin role succeeded")
	}
}

// The trigger must only guard system roles: an application role stays editable, and nobody can
// mint a system role or promote an application role to one.
func TestRF021_TriggerSoloProtegeRolesDelSistema(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)

	if _, err := pool.Exec(ctx, `UPDATE roles SET description = 'editada' WHERE name = 'contabilidad.analista'`); err != nil {
		t.Fatalf("UPDATE application role: %v", err)
	}
	var description string
	if err := pool.QueryRow(ctx, `SELECT description FROM roles WHERE name = 'contabilidad.analista'`).Scan(&description); err != nil {
		t.Fatalf("read application role: %v", err)
	}
	if description != "editada" {
		t.Fatalf("application role description=%q, want the update to persist", description)
	}
	if _, err := pool.Exec(ctx, `UPDATE roles SET system = true WHERE name = 'contabilidad.analista'`); err == nil {
		t.Fatal("promoting an application role to system succeeded")
	}
	if _, err := pool.Exec(ctx, `INSERT INTO roles (name, system) VALUES ('superadmin', true)`); err == nil {
		t.Fatal("INSERT of a new system role succeeded")
	}
}

func TestRF020_StoreResuelvePermisosPorRolesYAplicacion(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.New(repository).UpdateUser(ctx, admin.UpdateInput{ActorUserID: uuid.New(), UserID: uuid.New(), Roles: rolesPointer([]string{"user", "unknown.application"})}); !errors.Is(err, admin.ErrInvalidRole) {
		t.Fatalf("database role validation error=%v, want %v", err, admin.ErrInvalidRole)
	}
	user, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "permissions@example.test", PasswordHash: "$argon2id$fixed-test-value", DisplayName: "Permissions"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name IN ('contabilidad.senior', 'contabilidad.analista')`, user.ID); err != nil {
		t.Fatal(err)
	}
	client := oauth.Client{ID: "contabilidad", RedirectURI: "http://contabilidad.localhost:8080/oauth/callback"}
	service := oauth.New(repository.OAuthRepository(), client, &roleTestReader{}, time.Now)
	want := []string{"cierre.ejecutar", "movimientos.aprobar", "movimientos.registrar", "movimientos.ver_todos", "reportes.ver"}
	if got := exchangeRolePermissions(t, ctx, service, client, user.ID); !reflect.DeepEqual(got, want) {
		t.Fatalf("permissions=%v, want %v", got, want)
	}

	var foreignApplicationID, foreignPermissionID, seniorRoleID uuid.UUID
	if err := pool.QueryRow(ctx, `INSERT INTO applications (client_id, redirect_uri, allowed_origin, name) VALUES ('foreign', 'http://foreign.test/callback', 'http://foreign.test', 'Foreign') RETURNING id`).Scan(&foreignApplicationID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `INSERT INTO permissions (application_id, key, description) VALUES ($1, 'foreign.escalate', 'Foreign permission') RETURNING id`, foreignApplicationID).Scan(&foreignPermissionID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT id FROM roles WHERE name = 'contabilidad.senior'`).Scan(&seniorRoleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO role_permissions (role_id, permission_id) VALUES ($1, $2)`, seniorRoleID, foreignPermissionID); err != nil {
		t.Fatal(err)
	}
	if got := exchangeRolePermissions(t, ctx, service, client, user.ID); !reflect.DeepEqual(got, want) {
		t.Fatalf("foreign permission leaked: got %v, want %v", got, want)
	}
}

func assertSeededRolePermissions(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string, want []string) {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT p.key FROM role_permissions rp JOIN roles r ON r.id = rp.role_id JOIN permissions p ON p.id = rp.permission_id WHERE r.name = $1 ORDER BY p.key`, name)
	if err != nil {
		t.Fatalf("query permissions for %s: %v", name, err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatal(err)
		}
		got = append(got, key)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("permissions for %s=%v, want %v", name, got, want)
	}
}

func exchangeRolePermissions(t *testing.T, ctx context.Context, service *oauth.Service, client oauth.Client, userID uuid.UUID) []string {
	t.Helper()
	issued, err := service.Authorize(ctx, userID, oauth.AuthorizeInput{ClientID: client.ID, RedirectURI: client.RedirectURI, ResponseType: "code", State: "state", CodeChallenge: challengeForIntegration("verifier"), CodeChallengeMethod: "S256"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Exchange(ctx, oauth.ExchangeInput{Code: issued.Code, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeVerifier: "verifier"})
	if err != nil {
		t.Fatal(err)
	}
	return result.Permissions
}

func rolesPointer(value []string) *[]string { return &value }

type roleTestReader struct{ offset byte }

func (r *roleTestReader) Read(p []byte) (int, error) {
	r.offset++
	for index := range p {
		p[index] = byte(index+1) + r.offset
	}
	return len(p), nil
}
