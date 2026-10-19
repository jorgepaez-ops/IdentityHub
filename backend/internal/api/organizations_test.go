package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/admin"
	"github.com/jorgepaez/identity-hub/internal/auth/employee"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type directoryStub struct {
	organizations []store.Organization
	applications  []store.UserApplication
	userID        uuid.UUID
	err           error
}

func (d *directoryStub) ListOrganizations(context.Context) ([]store.Organization, error) {
	return d.organizations, d.err
}
func (d *directoryStub) ListUserApplications(_ context.Context, userID uuid.UUID) ([]store.UserApplication, error) {
	d.userID = userID
	return d.applications, d.err
}

func directoryServer(t *testing.T, actor uuid.UUID, roles []string, directory organizationDirectory) *Server {
	t.Helper()
	server := NewServer(nil, "test", nil)
	server.SetTokenService(apiTestService(t))
	server.SetCurrentUserRepository(&rbacRepositoryStub{user: store.User{ID: actor, Status: "active"}, roles: roles})
	if directory != nil {
		server.SetOrganizationDirectory(directory)
	}
	return server
}

func authorizedRequest(t *testing.T, server *Server, method, path, body string, actor uuid.UUID, roles ...string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	raw, err := apiTestService(t).Issue(actor.String(), roles)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+raw)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	return response
}

func TestRF022_ListaLasEmpresasYMarcaAlOperador(t *testing.T) {
	actor := uuid.New()
	directory := &directoryStub{organizations: []store.Organization{
		{ID: store.ContabilidadOrganizationID, Slug: "contabilidad-sa", Name: "Contabilidad S.A."},
		{ID: store.HubOrganizationID, Slug: "hub", Name: "Identity Hub (operador)"},
	}}
	server := directoryServer(t, actor, []string{"admin"}, directory)

	response := authorizedRequest(t, server, http.MethodGet, "/api/v1/admin/organizations", "", actor, "admin")

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body []Organization
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 2 || body[0].Slug != "contabilidad-sa" || body[0].Operator || body[1].Slug != "hub" || !body[1].Operator {
		t.Fatalf("body=%+v, want the operator flagged only on hub", body)
	}
}

func TestRF022_ListaDeEmpresasSoloParaAdministradores(t *testing.T) {
	actor := uuid.New()
	server := directoryServer(t, actor, []string{"user"}, &directoryStub{})

	if response := authorizedRequest(t, server, http.MethodGet, "/api/v1/admin/organizations", "", actor, "user"); response.Code != http.StatusForbidden {
		t.Fatalf("non-admin status=%d, want 403", response.Code)
	}
	anonymous := httptest.NewRecorder()
	server.Routes().ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations", nil))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d, want 401", anonymous.Code)
	}
	unwired := directoryServer(t, actor, []string{"admin"}, nil)
	if response := authorizedRequest(t, unwired, http.MethodGet, "/api/v1/admin/organizations", "", actor, "admin"); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unwired status=%d, want 503", response.Code)
	}
	failing := directoryServer(t, actor, []string{"admin"}, &directoryStub{err: errors.New("database down")})
	if response := authorizedRequest(t, failing, http.MethodGet, "/api/v1/admin/organizations", "", actor, "admin"); response.Code != http.StatusInternalServerError {
		t.Fatalf("failing status=%d, want 500", response.Code)
	}
}

func TestRF022_ListadoDeUsuariosExigeYAplicaLaEmpresa(t *testing.T) {
	actor := uuid.New()
	service := &adminServiceStub{listUsers: []admin.User{}}
	server := adminServer(t, actor, service)

	missing := adminRequest(t, server, http.MethodGet, "/api/v1/admin/users", "", actor)
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("without organizationId status=%d, want 400", missing.Code)
	}
	if invalid := adminRequest(t, server, http.MethodGet, "/api/v1/admin/users?organizationId=not-a-uuid", "", actor); invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid organizationId status=%d, want 400", invalid.Code)
	}
	if zero := adminRequest(t, server, http.MethodGet, "/api/v1/admin/users?organizationId="+uuid.Nil.String(), "", actor); zero.Code != http.StatusBadRequest {
		t.Fatalf("nil organizationId status=%d, want 400", zero.Code)
	}

	response := adminRequest(t, server, http.MethodGet, "/api/v1/admin/users?organizationId="+store.EmpresaBOrganizationID.String(), "", actor)

	if response.Code != http.StatusOK || service.listInput.OrganizationID != store.EmpresaBOrganizationID {
		t.Fatalf("status=%d organization=%s, want the API to filter by Empresa B", response.Code, service.listInput.OrganizationID)
	}
}

func TestRF022_ListadoDeAplicacionesExigeYAplicaLaEmpresa(t *testing.T) {
	actor := uuid.New()
	service := &roleGridStub{}
	server := roleGridServer(t, actor, []string{"admin"}, service)

	if response := roleGridRequest(t, server, http.MethodGet, "/api/v1/admin/applications", "", actor, "admin"); response.Code != http.StatusBadRequest {
		t.Fatalf("without organizationId status=%d, want 400", response.Code)
	}

	response := roleGridRequest(t, server, http.MethodGet, "/api/v1/admin/applications?organizationId="+store.EmpresaBOrganizationID.String(), "", actor, "admin")

	if response.Code != http.StatusOK || service.listOrganization != store.EmpresaBOrganizationID {
		t.Fatalf("status=%d organization=%s, want the API to filter by Empresa B", response.Code, service.listOrganization)
	}
}

func TestRF022_AsignarRolDeOtraEmpresaDevuelve400(t *testing.T) {
	actor, target := uuid.New(), uuid.New()
	service := &adminServiceStub{err: admin.ErrCrossOrganizationRole}
	server := adminServer(t, actor, service)

	response := adminRequest(t, server, http.MethodPatch, "/api/v1/admin/users/"+target.String(), `{"roles":["user","contabilidad.analista"]}`, actor)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), problemRoleOrganizationMismatch) {
		t.Fatalf("status=%d body=%s, want 400 role-organization-mismatch", response.Code, response.Body.String())
	}
}

func TestRF022_AltaConRolDeOtraEmpresaDevuelve400(t *testing.T) {
	service := &employeeServiceStub{err: store.ErrCrossOrganizationRole}
	server, actor := employeeServer(t, []string{"admin"}, service)
	body := `{"email":"b@example.com","displayName":"B","organizationId":"` + store.EmpresaBOrganizationID.String() + `","roles":["contabilidad.analista"]}`

	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/users", body, actor)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), problemRoleOrganizationMismatch) {
		t.Fatalf("status=%d body=%s, want 400 role-organization-mismatch", response.Code, response.Body.String())
	}
	if service.input.OrganizationID != store.EmpresaBOrganizationID {
		t.Fatalf("organization=%s, want the one from the request body", service.input.OrganizationID)
	}
}

func TestRF022_AltaSinEmpresaDevuelve400(t *testing.T) {
	service := &employeeServiceStub{err: &employee.InvalidInputError{Field: "organizationId", Detail: "is required"}}
	server, actor := employeeServer(t, []string{"admin"}, service)

	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/users", `{"email":"b@example.com","displayName":"B","roles":["user"]}`, actor)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "organizationId") {
		t.Fatalf("status=%d body=%s, want 400 naming organizationId", response.Code, response.Body.String())
	}
}

func TestRF022_MisAplicacionesListaLasDelTitular(t *testing.T) {
	actor := uuid.New()
	directory := &directoryStub{applications: []store.UserApplication{{ClientID: "contabilidad", Name: "Contabilidad", LaunchURL: "http://contabilidad.localhost:8080"}}}
	server := directoryServer(t, actor, []string{"user", "contabilidad.analista"}, directory)

	response := authorizedRequest(t, server, http.MethodGet, "/api/v1/me/applications", "", actor, "user", "contabilidad.analista")

	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body []UserApplication
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body[0].ClientId != "contabilidad" || body[0].LaunchUrl != "http://contabilidad.localhost:8080" || directory.userID != actor {
		t.Fatalf("body=%+v user=%s, want the caller's applications", body, directory.userID)
	}
}

func TestRF022_MisAplicacionesSinRolesDevuelveListaVacia(t *testing.T) {
	actor := uuid.New()
	server := directoryServer(t, actor, []string{"admin"}, &directoryStub{})

	response := authorizedRequest(t, server, http.MethodGet, "/api/v1/me/applications", "", actor, "admin")

	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != "[]" {
		t.Fatalf("status=%d body=%s, want an empty JSON list", response.Code, response.Body.String())
	}
}

func TestRF022_MisAplicacionesExigeSesionYServicio(t *testing.T) {
	actor := uuid.New()
	server := directoryServer(t, actor, []string{"user"}, &directoryStub{})
	anonymous := httptest.NewRecorder()
	server.Routes().ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, "/api/v1/me/applications", nil))
	if anonymous.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status=%d, want 401", anonymous.Code)
	}
	unwired := directoryServer(t, actor, []string{"user"}, nil)
	if response := authorizedRequest(t, unwired, http.MethodGet, "/api/v1/me/applications", "", actor, "user"); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("unwired status=%d, want 503", response.Code)
	}
	failing := directoryServer(t, actor, []string{"user"}, &directoryStub{err: errors.New("database down")})
	if response := authorizedRequest(t, failing, http.MethodGet, "/api/v1/me/applications", "", actor, "user"); response.Code != http.StatusInternalServerError {
		t.Fatalf("failing status=%d, want 500", response.Code)
	}
}

// Authentication and the admin role are decided before the company parameter is validated (401, then 403,
// then 400), so a missing organizationId never masks an authorization failure.
func TestRF022_AutenticacionYRolPrecedenAlParametroDeEmpresa(t *testing.T) {
	actor := uuid.New()
	for _, path := range []string{"/api/v1/admin/users", "/api/v1/admin/applications"} {
		t.Run(path, func(t *testing.T) {
			admins := adminServer(t, actor, &adminServiceStub{})
			anonymous := httptest.NewRecorder()
			admins.Routes().ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, path, nil))
			if anonymous.Code != http.StatusUnauthorized {
				t.Fatalf("no token and no organizationId: status=%d, want 401", anonymous.Code)
			}

			nonAdmin := roleGridServer(t, actor, []string{"user"}, &roleGridStub{})
			if response := roleGridRequest(t, nonAdmin, http.MethodGet, path, "", actor, "user"); response.Code != http.StatusForbidden {
				t.Fatalf("non-admin token and no organizationId: status=%d, want 403", response.Code)
			}

			admin := roleGridServer(t, actor, []string{"admin"}, &roleGridStub{})
			admin.SetAdminUserService(&adminServiceStub{})
			if response := roleGridRequest(t, admin, http.MethodGet, path, "", actor, "admin"); response.Code != http.StatusBadRequest {
				t.Fatalf("admin token and no organizationId: status=%d, want 400", response.Code)
			}
		})
	}
}
