package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/rolegrid"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type roleGridStub struct {
	applications []rolegrid.Application
	roles        []rolegrid.Role
	role         rolegrid.Role
	err          error
	createInput  rolegrid.CreateInput
	updateInput  rolegrid.UpdateInput
	deleteInput  rolegrid.DeleteInput
}

func (s *roleGridStub) ListApplications(context.Context) ([]rolegrid.Application, error) {
	return s.applications, s.err
}
func (s *roleGridStub) ListRoles(context.Context, uuid.UUID) ([]rolegrid.Role, error) {
	return s.roles, s.err
}
func (s *roleGridStub) Create(_ context.Context, input rolegrid.CreateInput) (rolegrid.Role, error) {
	s.createInput = input
	return s.role, s.err
}
func (s *roleGridStub) Update(_ context.Context, input rolegrid.UpdateInput) (rolegrid.Role, error) {
	s.updateInput = input
	return s.role, s.err
}
func (s *roleGridStub) Delete(_ context.Context, input rolegrid.DeleteInput) error {
	s.deleteInput = input
	return s.err
}

func roleGridServer(t *testing.T, actor uuid.UUID, roles []string, service rolegrid.Manager) *Server {
	t.Helper()
	server := NewServer(nil, "test", nil)
	server.SetTokenService(apiTestService(t))
	server.SetCurrentUserRepository(&rbacRepositoryStub{user: store.User{ID: actor, Status: "active"}, roles: roles})
	if service != nil {
		server.SetRoleGridService(service)
	}
	return server
}

func roleGridRequest(t *testing.T, server *Server, method, path, body string, actor uuid.UUID, roles ...string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("User-Agent", "role-grid-tests")
	r.RemoteAddr = "203.0.113.7:4444"
	raw, err := apiTestService(t).Issue(actor.String(), roles)
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+raw)
	w := httptest.NewRecorder()
	server.Routes().ServeHTTP(w, r)
	return w
}

func TestRF021_ListaAplicacionesConPermisosYRoles(t *testing.T) {
	actor, appID, roleID := uuid.New(), uuid.New(), uuid.New()
	service := &roleGridStub{applications: []rolegrid.Application{{ID: appID, ClientID: "contabilidad", Name: "Contabilidad", Permissions: []rolegrid.Permission{{Key: "reportes.ver", Description: "Ver"}}, Roles: []rolegrid.Role{{ID: roleID, ApplicationID: appID, Name: "contabilidad.auditor", PermissionKeys: []string{"reportes.ver"}, AssignedCount: 2}}}}}

	response := roleGridRequest(t, roleGridServer(t, actor, []string{"admin"}, service), http.MethodGet, "/api/v1/admin/applications", "", actor, "admin")
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body []Application
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || body[0].ClientId != "contabilidad" || body[0].Name != "Contabilidad" || body[0].Permissions[0].Description != "Ver" || body[0].Roles[0].AssignedCount != 2 {
		t.Fatalf("body=%+v", body)
	}
}

func TestRF021_ListaRolesDeUnaAplicacion(t *testing.T) {
	actor, appID := uuid.New(), uuid.New()
	service := &roleGridStub{roles: []rolegrid.Role{{ID: uuid.New(), Name: "contabilidad.auditor"}}}
	server := roleGridServer(t, actor, []string{"admin"}, service)

	response := roleGridRequest(t, server, http.MethodGet, "/api/v1/admin/applications/"+appID.String()+"/roles", "", actor, "admin")
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "contabilidad.auditor") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}

	service.err = rolegrid.ErrApplicationNotFound
	response = roleGridRequest(t, server, http.MethodGet, "/api/v1/admin/applications/"+appID.String()+"/roles", "", actor, "admin")
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", response.Code)
	}
}

func TestRF021_CreaRolConIPYAgenteDeUsuarioReales(t *testing.T) {
	actor, appID := uuid.New(), uuid.New()
	service := &roleGridStub{role: rolegrid.Role{ID: uuid.New(), ApplicationID: appID, Name: "contabilidad.auditor", PermissionKeys: []string{"reportes.ver"}}}
	body := `{"name":"contabilidad.auditor","description":"Solo lectura","permissionKeys":["reportes.ver"]}`

	response := roleGridRequest(t, roleGridServer(t, actor, []string{"admin"}, service), http.MethodPost, "/api/v1/admin/applications/"+appID.String()+"/roles", body, actor, "admin")
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	input := service.createInput
	if input.ActorUserID != actor || input.ApplicationID != appID || input.Name != "contabilidad.auditor" || input.Description != "Solo lectura" || input.UserAgent != "role-grid-tests" || input.IP.String() != "203.0.113.7" {
		t.Fatalf("input=%+v", input)
	}
}

func TestRF021_MapeaLosErroresDelServicioAEstadosHTTP(t *testing.T) {
	tests := []struct {
		err  error
		want int
	}{
		{rolegrid.ErrInvalidRoleName, http.StatusBadRequest},
		{rolegrid.ErrInvalidPermission, http.StatusBadRequest},
		{rolegrid.ErrInvalidDescription, http.StatusBadRequest},
		{rolegrid.ErrEmptyUpdate, http.StatusBadRequest},
		{rolegrid.ErrSelfPermissionChange, http.StatusForbidden},
		{rolegrid.ErrSelfRoleDelete, http.StatusForbidden},
		{rolegrid.ErrSystemRole, http.StatusForbidden},
		{rolegrid.ErrApplicationNotFound, http.StatusNotFound},
		{rolegrid.ErrRoleNotFound, http.StatusNotFound},
		{rolegrid.ErrDuplicateRole, http.StatusConflict},
		{rolegrid.ErrRoleAssigned, http.StatusConflict},
		{errors.New("database down"), http.StatusInternalServerError},
	}
	actor, appID, roleID := uuid.New(), uuid.New(), uuid.New()
	base := "/api/v1/admin/applications/" + appID.String() + "/roles"
	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			server := roleGridServer(t, actor, []string{"admin"}, &roleGridStub{err: tt.err})
			for _, call := range []struct{ method, path, body string }{
				{http.MethodPost, base, `{"name":"contabilidad.auditor","permissionKeys":[]}`},
				{http.MethodPatch, base + "/" + roleID.String(), `{"description":"x"}`},
				{http.MethodDelete, base + "/" + roleID.String(), ``},
			} {
				response := roleGridRequest(t, server, call.method, call.path, call.body, actor, "admin")
				if response.Code != tt.want {
					t.Fatalf("%s: status=%d, want %d body=%s", call.method, response.Code, tt.want, response.Body.String())
				}
				if !strings.HasPrefix(response.Header().Get("Content-Type"), "application/problem+json") {
					t.Fatalf("%s: content-type=%q, want a problem", call.method, response.Header().Get("Content-Type"))
				}
			}
		})
	}
}

func TestRF021_ActualizaYEliminaRol(t *testing.T) {
	actor, appID, roleID := uuid.New(), uuid.New(), uuid.New()
	service := &roleGridStub{role: rolegrid.Role{ID: roleID, ApplicationID: appID, Name: "contabilidad.auditor", Description: "Nueva", PermissionKeys: []string{"reportes.ver"}}}
	server := roleGridServer(t, actor, []string{"admin"}, service)
	path := "/api/v1/admin/applications/" + appID.String() + "/roles/" + roleID.String()

	response := roleGridRequest(t, server, http.MethodPatch, path, `{"description":"Nueva","permissionKeys":["reportes.ver"]}`, actor, "admin")
	if response.Code != http.StatusOK {
		t.Fatalf("PATCH status=%d body=%s", response.Code, response.Body.String())
	}
	update := service.updateInput
	if update.ActorUserID != actor || update.RoleID != roleID || update.ApplicationID != appID || *update.Description != "Nueva" || len(*update.PermissionKeys) != 1 || update.IP.String() != "203.0.113.7" || update.UserAgent != "role-grid-tests" {
		t.Fatalf("update input=%+v", update)
	}

	response = roleGridRequest(t, server, http.MethodDelete, path, "", actor, "admin")
	if response.Code != http.StatusNoContent || service.deleteInput.RoleID != roleID || service.deleteInput.IP.String() != "203.0.113.7" {
		t.Fatalf("DELETE status=%d input=%+v", response.Code, service.deleteInput)
	}
}

func TestRF021_ActualizaRechazaCuerposInvalidos(t *testing.T) {
	actor, appID, roleID := uuid.New(), uuid.New(), uuid.New()
	server := roleGridServer(t, actor, []string{"admin"}, &roleGridStub{})
	path := "/api/v1/admin/applications/" + appID.String() + "/roles/" + roleID.String()
	for name, body := range map[string]string{
		"JSON roto":               `{`,
		"renombrar no se permite": `{"name":"contabilidad.otro"}`,
		"campo desconocido":       `{"permissionKeys":[],"extra":1}`,
		"cuerpo vacio":            `{}`,
		"crear con JSON roto":     `nope`,
	} {
		method, target := http.MethodPatch, path
		if name == "crear con JSON roto" {
			method, target = http.MethodPost, "/api/v1/admin/applications/"+appID.String()+"/roles"
		}
		response := roleGridRequest(t, server, method, target, body, actor, "admin")
		if response.Code != http.StatusBadRequest {
			t.Fatalf("%s: status=%d body=%s", name, response.Code, response.Body.String())
		}
	}
}

func TestRF021_SoloAdminAccedeALaGrilla(t *testing.T) {
	actor, appID, roleID := uuid.New(), uuid.New(), uuid.New()
	service := &roleGridStub{}
	server := roleGridServer(t, actor, []string{"user", "contabilidad.senior"}, service)
	base := "/api/v1/admin/applications"
	calls := []struct{ method, path, body string }{
		{http.MethodGet, base, ""},
		{http.MethodGet, base + "/" + appID.String() + "/roles", ""},
		{http.MethodPost, base + "/" + appID.String() + "/roles", `{"name":"contabilidad.x1","permissionKeys":[]}`},
		{http.MethodPatch, base + "/" + appID.String() + "/roles/" + roleID.String(), `{"description":"x"}`},
		{http.MethodDelete, base + "/" + appID.String() + "/roles/" + roleID.String(), ""},
	}
	for _, call := range calls {
		response := roleGridRequest(t, server, call.method, call.path, call.body, actor, "user", "contabilidad.senior")
		if response.Code != http.StatusForbidden {
			t.Fatalf("%s %s: status=%d, want 403", call.method, call.path, response.Code)
		}
		anonymous := httptest.NewRecorder()
		server.Routes().ServeHTTP(anonymous, httptest.NewRequest(call.method, call.path, strings.NewReader(call.body)))
		if anonymous.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s: anonymous status=%d, want 401", call.method, call.path, anonymous.Code)
		}
	}
	if service.createInput.Name != "" || service.deleteInput.RoleID != uuid.Nil {
		t.Fatal("the service ran for a non-admin")
	}
}

func TestRF021_SinServicioDevuelve503(t *testing.T) {
	actor, appID, roleID := uuid.New(), uuid.New(), uuid.New()
	server := roleGridServer(t, actor, []string{"admin"}, nil)
	base := "/api/v1/admin/applications"
	for _, call := range []struct{ method, path, body string }{
		{http.MethodGet, base, ""},
		{http.MethodGet, base + "/" + appID.String() + "/roles", ""},
		{http.MethodPost, base + "/" + appID.String() + "/roles", `{}`},
		{http.MethodPatch, base + "/" + appID.String() + "/roles/" + roleID.String(), `{}`},
		{http.MethodDelete, base + "/" + appID.String() + "/roles/" + roleID.String(), ""},
	} {
		if response := roleGridRequest(t, server, call.method, call.path, call.body, actor, "admin"); response.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s %s: status=%d, want 503", call.method, call.path, response.Code)
		}
	}
}

func TestRF021_ErrorInesperadoSeRegistraConRequestIDSinFiltrarseAlCliente(t *testing.T) {
	actor, appID, roleID := uuid.New(), uuid.New(), uuid.New()
	base := "/api/v1/admin/applications"
	roleBase := base + "/" + appID.String() + "/roles"
	for _, call := range []struct{ method, path, body string }{
		{http.MethodGet, base, ``},
		{http.MethodGet, roleBase, ``},
		{http.MethodPost, roleBase, `{"name":"contabilidad.auditor","permissionKeys":[]}`},
		{http.MethodPatch, roleBase + "/" + roleID.String(), `{"description":"x"}`},
		{http.MethodDelete, roleBase + "/" + roleID.String(), ``},
	} {
		t.Run(call.method+" "+call.path, func(t *testing.T) {
			var logs bytes.Buffer
			server := roleGridServer(t, actor, []string{"admin"}, &roleGridStub{err: errors.New("pq: secret-detail unavailable")})
			server.logger = slog.New(slog.NewTextHandler(&logs, nil))
			r := httptest.NewRequest(call.method, call.path, strings.NewReader(call.body))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("X-Request-Id", "role-grid-request-id")
			raw, err := apiTestService(t).Issue(actor.String(), []string{"admin"})
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set("Authorization", "Bearer "+raw)
			response := httptest.NewRecorder()
			server.Routes().ServeHTTP(response, r)
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status=%d body=%s, want 500", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "secret-detail") {
				t.Fatalf("response leaks the underlying error: %s", response.Body.String())
			}
			if output := logs.String(); !strings.Contains(output, "level=ERROR") || !strings.Contains(output, "secret-detail") || !strings.Contains(output, "request_id=role-grid-request-id") {
				t.Fatalf("logs=%q", output)
			}
		})
	}
}
