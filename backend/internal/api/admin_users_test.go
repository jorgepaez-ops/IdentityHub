package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/admin"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type adminServiceStub struct {
	listInput   admin.ListInput
	getUser     admin.User
	listUsers   []admin.User
	updateInput admin.UpdateInput
	updateUser  admin.User
	err         error
}

func (s *adminServiceStub) ListUsers(_ context.Context, input admin.ListInput) ([]admin.User, error) {
	s.listInput = input
	return s.listUsers, s.err
}
func (s *adminServiceStub) GetUser(context.Context, uuid.UUID) (admin.User, error) {
	return s.getUser, s.err
}
func (s *adminServiceStub) UpdateUser(_ context.Context, input admin.UpdateInput) (admin.User, error) {
	s.updateInput = input
	return s.updateUser, s.err
}

func adminServer(t *testing.T, actor uuid.UUID, service admin.Manager) *Server {
	t.Helper()
	server := NewServer(nil, "test", nil)
	server.SetTokenService(apiTestService(t))
	server.SetCurrentUserRepository(&rbacRepositoryStub{user: store.User{ID: actor, Status: "active"}, roles: []string{"admin"}})
	server.SetAdminUserService(service)
	return server
}

func adminRequest(t *testing.T, server *Server, method, path string, body string, actor uuid.UUID) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, nil)
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	raw, err := apiTestService(t).Issue(actor.String(), []string{"admin"})
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+raw)
	w := httptest.NewRecorder()
	server.Routes().ServeHTTP(w, r)
	return w
}

func TestRF010_BusquedaNoEsInyectable(t *testing.T) {
	actor := uuid.New()
	service := &adminServiceStub{listUsers: []admin.User{}}
	response := adminRequest(t, adminServer(t, actor, service), http.MethodGet, "/api/v1/admin/users?q=%27%20OR%20%271%27%3D%271%27%20--", "", actor)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if service.listInput.Query != "' OR '1'='1' --" {
		t.Fatalf("q=%q must be passed as a literal parameter", service.listInput.Query)
	}
	var page UserPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 0 {
		t.Fatalf("items=%v, injection text must not match every user", page.Items)
	}
}

func TestRF010_AdminNoPuedeDeshabilitarseASiMismo(t *testing.T) {
	actor := uuid.New()
	service := &adminServiceStub{err: admin.ErrSelfDisable}
	response := adminRequest(t, adminServer(t, actor, service), http.MethodPatch, "/api/v1/admin/users/"+actor.String(), `{"status":"disabled"}`, actor)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRF010_AdminNoPuedeAsignarseRolesASiMismo(t *testing.T) {
	actor := uuid.New()
	service := &adminServiceStub{err: admin.ErrSelfRoleAssignment}
	response := adminRequest(t, adminServer(t, actor, service), http.MethodPatch, "/api/v1/admin/users/"+actor.String(), `{"roles":["contabilidad.senior"]}`, actor)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRF009_UpdateUserRechazaRolDesconocido(t *testing.T) {
	actor, target := uuid.New(), uuid.New()
	service := &adminServiceStub{err: admin.ErrInvalidRole}
	response := adminRequest(t, adminServer(t, actor, service), http.MethodPatch, "/api/v1/admin/users/"+target.String(), `{"roles":["operator"]}`, actor)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRF009_UpdateUserRechazaQuitarElRolBaseUser(t *testing.T) {
	actor, target := uuid.New(), uuid.New()
	service := &adminServiceStub{err: admin.ErrBaseRoleRequired}
	response := adminRequest(t, adminServer(t, actor, service), http.MethodPatch, "/api/v1/admin/users/"+target.String(), `{"roles":["contabilidad.senior"]}`, actor)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRF009_UpdateUserAceptaRolesDeNegocioDeContabilidad(t *testing.T) {
	actor, target := uuid.New(), uuid.New()
	service := &adminServiceStub{updateUser: admin.User{ID: target, Email: "target@example.test", Status: admin.StatusActive, Roles: []string{"user", "contabilidad.senior"}}}
	response := adminRequest(t, adminServer(t, actor, service), http.MethodPatch, "/api/v1/admin/users/"+target.String(), `{"roles":["user","contabilidad.senior"]}`, actor)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body User
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, role := range body.Roles {
		if string(role) == "contabilidad.senior" {
			found = true
		}
	}
	if !found {
		t.Fatalf("roles=%v, want contabilidad.senior included", body.Roles)
	}
}
