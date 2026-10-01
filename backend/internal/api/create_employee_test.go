package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/employee"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type employeeServiceStub struct {
	input  employee.Input
	result employee.Result
	err    error
}

func (s *employeeServiceStub) CreateEmployee(_ context.Context, input employee.Input) (employee.Result, error) {
	s.input = input
	return s.result, s.err
}

func employeeServer(t *testing.T, actorRoles []string, service employee.Creator) (*Server, uuid.UUID) {
	t.Helper()
	actor := uuid.New()
	server := NewServer(nil, "test", nil)
	server.SetTokenService(apiTestService(t))
	server.SetCurrentUserRepository(&rbacRepositoryStub{user: store.User{ID: actor, Status: "active"}, roles: actorRoles})
	server.SetEmployeeCreationService(service)
	return server, actor
}

func TestRF001_CreateEmployeeDevuelve201ConRolBaseYRolesSolicitados(t *testing.T) {
	target := uuid.New()
	service := &employeeServiceStub{result: employee.Result{ID: target, Email: "ana@example.com", DisplayName: "Ana", Status: "pending_verification", Roles: []string{"user", "contabilidad.analista"}}}
	server, actor := employeeServer(t, []string{"admin"}, service)
	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/users", `{"email":"ana@example.com","displayName":"Ana","roles":["contabilidad.analista"]}`, actor)
	if response.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body User
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Status != PendingVerification {
		t.Fatalf("status = %q, want pending_verification", body.Status)
	}
	found := map[string]bool{}
	for _, role := range body.Roles {
		found[string(role)] = true
	}
	if !found["user"] || !found["contabilidad.analista"] {
		t.Fatalf("roles = %v, want user and contabilidad.analista", body.Roles)
	}
	if service.input.Email != "ana@example.com" || service.input.ActorUserID != actor {
		t.Fatalf("service received input = %+v, want email/actor from the request", service.input)
	}
}

func TestRF001_CreateEmployeeSinAuthDevuelve401(t *testing.T) {
	server, _ := employeeServer(t, []string{"admin"}, &employeeServiceStub{})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", strings.NewReader(`{"email":"ana@example.com","displayName":"Ana","roles":["user"]}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRF001_CreateEmployeeSinRolAdminDevuelve403(t *testing.T) {
	service := &employeeServiceStub{}
	server, actor := employeeServer(t, []string{"user"}, service)
	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/users", `{"email":"ana@example.com","displayName":"Ana","roles":["user"]}`, actor)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if service.input.Email != "" {
		t.Fatal("employee service was called despite a non-admin actor")
	}
}

func TestRF001_CreateEmployeeRolDesconocidoDevuelve400(t *testing.T) {
	service := &employeeServiceStub{err: &employee.InvalidInputError{Field: "roles", Detail: `unknown role "operator"`}}
	server, actor := employeeServer(t, []string{"admin"}, service)
	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/users", `{"email":"ana@example.com","displayName":"Ana","roles":["operator"]}`, actor)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRF001_CreateEmployeeCorreoDuplicadoDevuelve409SinRevelarExistencia(t *testing.T) {
	service := &employeeServiceStub{err: employee.ErrEmailExists}
	server, actor := employeeServer(t, []string{"admin"}, service)
	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/users", `{"email":"ana@example.com","displayName":"Ana","roles":["user"]}`, actor)
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if len(response.Body.Bytes()) == 0 {
		t.Fatal("expected a problem+json body")
	}
	var problem Problem
	if err := json.Unmarshal(response.Body.Bytes(), &problem); err != nil {
		t.Fatal(err)
	}
	if problem.Detail != nil && *problem.Detail == "ana@example.com" {
		t.Fatalf("response revealed the existing account: %+v", problem)
	}
}

func TestRF001_CreateEmployeeFalloDelBrokerDevuelve503(t *testing.T) {
	service := &employeeServiceStub{err: employee.ErrPublish}
	server, actor := employeeServer(t, []string{"admin"}, service)
	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/users", `{"email":"ana@example.com","displayName":"Ana","roles":["user"]}`, actor)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
