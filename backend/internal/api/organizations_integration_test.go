//go:build integration

package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jorgepaez/identity-hub/internal/auth/admin"
	"github.com/jorgepaez/identity-hub/internal/auth/employee"
	"github.com/jorgepaez/identity-hub/internal/auth/rolegrid"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

type companyFixture struct {
	pool       *pgxpool.Pool
	repository *store.Store
	server     *Server
	adminID    uuid.UUID
}

// newCompanyFixture composes the real store behind Server.Routes(), like cmd/api, with an active
// global administrator that belongs to the operator company.
func newCompanyFixture(t *testing.T) companyFixture {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatal(err)
	}
	administrator := companyUser(t, repository, pool, store.HubOrganizationID, "company-admin@example.test", "admin")
	server := NewServer(nil, "test", nil)
	server.SetTokenService(apiTestService(t))
	server.SetCurrentUserRepository(repository)
	server.SetAdminUserService(admin.New(repository))
	server.SetRoleGridService(rolegrid.New(repository))
	server.SetOrganizationDirectory(repository)
	server.SetEmployeeCreationService(employee.New(repository, resetPublisher{}, resetHasher{}, rand.Reader, time.Now))
	return companyFixture{pool: pool, repository: repository, server: server, adminID: administrator}
}

// companyUser creates an active user of the organization holding the given roles (plus the base role).
func companyUser(t *testing.T, repository *store.Store, pool *pgxpool.Pool, organizationID uuid.UUID, email string, roles ...string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	user, err := repository.CreateUser(ctx, store.CreateUserParams{OrganizationID: organizationID, Email: email, PasswordHash: "$argon2id$placeholder", DisplayName: email})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, user.ID); err != nil {
		t.Fatal(err)
	}
	for _, role := range append([]string{"user"}, roles...) {
		if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) SELECT $1, id FROM roles WHERE name = $2`, user.ID, role); err != nil {
			t.Fatal(err)
		}
	}
	return user.ID
}

func (f companyFixture) call(t *testing.T, method, path, body string, actor uuid.UUID, roles ...string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	raw, err := apiTestService(t).Issue(actor.String(), roles)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+raw)
	response := httptest.NewRecorder()
	f.server.Routes().ServeHTTP(response, request)
	return response
}

func (f companyFixture) callAsAdmin(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return f.call(t, method, path, body, f.adminID, "admin", "user")
}

func (f companyFixture) auditCount(t *testing.T, action, reason string) int {
	t.Helper()
	var count int
	err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_log WHERE action = $1 AND actor_user_id = $2 AND metadata->>'reason' = $3`, action, f.adminID, reason).Scan(&count)
	if err != nil {
		t.Fatal(err)
	}
	return count
}

func TestRF022_ConsolaListaEmpresasUsuariosYAplicacionesPorEmpresaConPostgres(t *testing.T) {
	f := newCompanyFixture(t)
	companyUser(t, f.repository, f.pool, store.ContabilidadOrganizationID, "conta-list@example.test", "contabilidad.analista")
	companyUser(t, f.repository, f.pool, store.EmpresaBOrganizationID, "empresab-list@example.test", "inventario.consulta")

	var organizations []Organization
	response := f.callAsAdmin(t, http.MethodGet, "/api/v1/admin/organizations", "")
	if err := json.Unmarshal(response.Body.Bytes(), &organizations); err != nil || response.Code != http.StatusOK {
		t.Fatalf("organizations status=%d err=%v body=%s", response.Code, err, response.Body.String())
	}
	operators := 0
	for _, organization := range organizations {
		if organization.Operator {
			operators++
			if organization.Slug != "hub" {
				t.Fatalf("operator = %+v, want only hub", organization)
			}
		}
	}
	if len(organizations) != 3 || operators != 1 {
		t.Fatalf("organizations=%+v", organizations)
	}

	var page UserPage
	response = f.callAsAdmin(t, http.MethodGet, "/api/v1/admin/users?organizationId="+store.EmpresaBOrganizationID.String(), "")
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != http.StatusOK {
		t.Fatalf("users status=%d err=%v body=%s", response.Code, err, response.Body.String())
	}
	if len(page.Items) != 1 || string(page.Items[0].Email) != "empresab-list@example.test" {
		t.Fatalf("Empresa B users = %+v, want only its own user", page.Items)
	}

	var applications []Application
	response = f.callAsAdmin(t, http.MethodGet, "/api/v1/admin/applications?organizationId="+store.EmpresaBOrganizationID.String(), "")
	if err := json.Unmarshal(response.Body.Bytes(), &applications); err != nil || response.Code != http.StatusOK {
		t.Fatalf("applications status=%d err=%v body=%s", response.Code, err, response.Body.String())
	}
	if len(applications) != 1 || applications[0].ClientId != "inventario" {
		t.Fatalf("Empresa B applications = %+v, want only inventario", applications)
	}
	for _, role := range applications[0].Roles {
		if !strings.HasPrefix(role.Name, "inventario.") {
			t.Fatalf("role %q leaked into Empresa B's grid", role.Name)
		}
	}
}

func TestRF022_AsignarRolDeOtraEmpresaPorLaAPIDevuelve400YAudita(t *testing.T) {
	f := newCompanyFixture(t)
	target := companyUser(t, f.repository, f.pool, store.EmpresaBOrganizationID, "empresab-assign@example.test", "inventario.consulta")

	response := f.callAsAdmin(t, http.MethodPatch, "/api/v1/admin/users/"+target.String(), `{"roles":["user","contabilidad.analista"]}`)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", response.Code, response.Body.String())
	}
	roles, err := f.repository.ListRolesForUser(context.Background(), target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(roles, ",") != "inventario.consulta,user" {
		t.Fatalf("roles=%v, want the previous roles kept", roles)
	}
	if got := f.auditCount(t, "role_assignment_rejected", "organization_mismatch"); got != 1 {
		t.Fatalf("role_assignment_rejected audits=%d, want 1", got)
	}

	if ok := f.callAsAdmin(t, http.MethodPatch, "/api/v1/admin/users/"+target.String(), `{"roles":["user","inventario.bodeguero"]}`); ok.Code != http.StatusOK {
		t.Fatalf("same-company assignment status=%d body=%s", ok.Code, ok.Body.String())
	}
}

func TestRF022_InvitarConRolDeOtraEmpresaDevuelve400YNoCreaLaCuenta(t *testing.T) {
	f := newCompanyFixture(t)
	body := func(email, role string) string {
		return `{"email":"` + email + `","displayName":"Invitado","organizationId":"` + store.EmpresaBOrganizationID.String() + `","roles":["` + role + `"]}`
	}

	rejected := f.callAsAdmin(t, http.MethodPost, "/api/v1/admin/users", body("invite-cross@example.test", "contabilidad.analista"))

	if rejected.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", rejected.Code, rejected.Body.String())
	}
	var accounts int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM users WHERE email = 'invite-cross@example.test'`).Scan(&accounts); err != nil || accounts != 0 {
		t.Fatalf("accounts=%d err=%v, want the rejected invitation to roll back", accounts, err)
	}
	if got := f.auditCount(t, "role_assignment_rejected", "organization_mismatch"); got != 1 {
		t.Fatalf("role_assignment_rejected audits=%d, want 1", got)
	}

	if created := f.callAsAdmin(t, http.MethodPost, "/api/v1/admin/users", body("invite-ok@example.test", "inventario.consulta")); created.Code != http.StatusCreated {
		t.Fatalf("same-company invitation status=%d body=%s", created.Code, created.Body.String())
	}
	var organization uuid.UUID
	if err := f.pool.QueryRow(context.Background(), `SELECT organization_id FROM users WHERE email = 'invite-ok@example.test'`).Scan(&organization); err != nil || organization != store.EmpresaBOrganizationID {
		t.Fatalf("organization=%s err=%v, want Empresa B", organization, err)
	}

	unknown := `{"email":"invite-unknown@example.test","displayName":"X","organizationId":"` + uuid.NewString() + `","roles":["user"]}`
	if response := f.callAsAdmin(t, http.MethodPost, "/api/v1/admin/users", unknown); response.Code != http.StatusBadRequest {
		t.Fatalf("unknown company status=%d body=%s, want 400", response.Code, response.Body.String())
	}
	missing := `{"email":"invite-missing@example.test","displayName":"X","roles":["user"]}`
	if response := f.callAsAdmin(t, http.MethodPost, "/api/v1/admin/users", missing); response.Code != http.StatusBadRequest {
		t.Fatalf("missing company status=%d body=%s, want 400", response.Code, response.Body.String())
	}
}

func TestRF022_MisAplicacionesSoloListaLasDeSuEmpresaConRol(t *testing.T) {
	f := newCompanyFixture(t)
	conta := companyUser(t, f.repository, f.pool, store.ContabilidadOrganizationID, "me-conta@example.test", "contabilidad.analista")
	bodeguero := companyUser(t, f.repository, f.pool, store.EmpresaBOrganizationID, "me-b@example.test", "inventario.bodeguero")
	withoutRole := companyUser(t, f.repository, f.pool, store.EmpresaBOrganizationID, "me-norole@example.test")

	names := func(user uuid.UUID, roles ...string) []string {
		response := f.call(t, http.MethodGet, "/api/v1/me/applications", "", user, roles...)
		var applications []UserApplication
		if err := json.Unmarshal(response.Body.Bytes(), &applications); err != nil || response.Code != http.StatusOK {
			t.Fatalf("status=%d err=%v body=%s", response.Code, err, response.Body.String())
		}
		result := make([]string, 0, len(applications))
		for _, application := range applications {
			result = append(result, application.ClientId+"@"+application.LaunchUrl)
		}
		return result
	}

	if got := names(conta, "user", "contabilidad.analista"); strings.Join(got, ",") != "contabilidad@http://contabilidad.localhost:8080" {
		t.Fatalf("Contabilidad user sees %v", got)
	}
	if got := names(bodeguero, "user", "inventario.bodeguero"); len(got) != 1 || !strings.HasPrefix(got[0], "inventario@") {
		t.Fatalf("Empresa B user sees %v", got)
	}
	if got := names(withoutRole, "user"); len(got) != 0 {
		t.Fatalf("user without application roles sees %v, want none", got)
	}
	if got := names(f.adminID, "admin", "user"); len(got) != 0 {
		t.Fatalf("global admin without application roles sees %v, want none", got)
	}
}
