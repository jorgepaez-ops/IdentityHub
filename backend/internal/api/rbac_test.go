package api

import (
	"context"
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type rbacRepositoryStub struct {
	user          store.User
	roles         []string
	getUserCalls  int
	listRoleCalls int
}

func (s *rbacRepositoryStub) GetUserByID(_ context.Context, id uuid.UUID) (store.User, error) {
	s.getUserCalls++
	if id != s.user.ID {
		return store.User{}, http.ErrNoLocation
	}
	return s.user, nil
}

func (s *rbacRepositoryStub) ListRolesForUser(_ context.Context, id uuid.UUID) ([]string, error) {
	s.listRoleCalls++
	if id != s.user.ID {
		return nil, http.ErrNoLocation
	}
	return append([]string(nil), s.roles...), nil
}

func (s *rbacRepositoryStub) UpdateDisplayName(_ context.Context, _ uuid.UUID, _ string) (store.User, error) {
	return s.user, nil
}

func rbacTestUser() store.User {
	return store.User{ID: uuid.New(), Status: "active"}
}

func rbacRequest(t *testing.T, method, path, raw string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(method, path, nil)
	request.Header.Set("Authorization", "Bearer "+raw)
	return request
}

func TestRF009_UserRecibe403EnRutasAdmin(t *testing.T) {
	user := rbacTestUser()
	repository := &rbacRepositoryStub{user: user, roles: []string{"user"}}
	server := NewServer(nil, "test", nil)
	server.SetTokenService(apiTestService(t))
	server.SetCurrentUserRepository(repository)

	raw, err := apiTestService(t).Issue(user.ID.String(), []string{"admin"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	for _, testCase := range []struct {
		name   string
		method string
		path   string
	}{
		{name: "list users", method: http.MethodGet, path: "/api/v1/admin/users"},
		{name: "get user", method: http.MethodGet, path: "/api/v1/admin/users/00000000-0000-0000-0000-000000000001"},
		{name: "update user", method: http.MethodPatch, path: "/api/v1/admin/users/00000000-0000-0000-0000-000000000001"},
		{name: "list audit log", method: http.MethodGet, path: "/api/v1/admin/audit-log"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			server.Routes().ServeHTTP(response, rbacRequest(t, testCase.method, testCase.path, raw))
			if response.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestRF009_AdminRecibe200(t *testing.T) {
	user := rbacTestUser()
	repository := &rbacRepositoryStub{user: user, roles: []string{"admin"}}
	service := apiTestService(t)
	raw, err := service.Issue(user.ID.String(), []string{"user"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	handler := RequireAuth(service)(RequireRole(repository, "admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, rbacRequest(t, http.MethodGet, "/admin", raw))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
}

func TestRF009_UsuarioNoActivoRecibe401(t *testing.T) {
	for _, status := range []string{"locked", "disabled", "pending_verification"} {
		t.Run(status, func(t *testing.T) {
			user := rbacTestUser()
			user.Status = status
			repository := &rbacRepositoryStub{user: user, roles: []string{"admin"}}
			service := apiTestService(t)
			raw, err := service.Issue(user.ID.String(), []string{"admin"})
			if err != nil {
				t.Fatalf("Issue: %v", err)
			}

			handler := RequireAuth(service)(RequireRole(repository, "admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, rbacRequest(t, http.MethodGet, "/admin", raw))
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestRF009_RolesAlteradosEnElTokenDanEl401(t *testing.T) {
	user := rbacTestUser()
	repository := &rbacRepositoryStub{user: user, roles: []string{"admin"}}
	serverTokens := apiTestService(t)
	forged := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"iss": "issuer", "aud": "audience", "sub": user.ID.String(), "roles": []string{"admin"},
		"iat": time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Unix(),
		"exp": time.Date(2026, 1, 2, 3, 19, 5, 0, time.UTC).Unix(),
	})
	forged.Header["kid"] = serverTokens.KeyID()
	raw, err := forged.SignedString(ed25519.NewKeyFromSeed([]byte("abcdefghijklmnopqrstuvwxyz123456")))
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	handler := RequireAuth(serverTokens)(RequireRole(repository, "admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, rbacRequest(t, http.MethodGet, "/admin", raw))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", response.Code, response.Body.String())
	}
	if repository.getUserCalls != 0 || repository.listRoleCalls != 0 {
		t.Fatalf("RequireRole was reached: GetUserByID=%d ListRolesForUser=%d", repository.getUserCalls, repository.listRoleCalls)
	}
}

func TestRF009_CambioDeRolEnBDRevocaAccesoEnSiguientePeticion(t *testing.T) {
	user := rbacTestUser()
	repository := &rbacRepositoryStub{user: user, roles: []string{"admin"}}
	service := apiTestService(t)
	raw, err := service.Issue(user.ID.String(), []string{"admin"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	handler := RequireAuth(service)(RequireRole(repository, "admin")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, rbacRequest(t, http.MethodGet, "/admin", raw))
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200: %s", first.Code, first.Body.String())
	}

	repository.roles = []string{"user"}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, rbacRequest(t, http.MethodGet, "/admin", raw))
	if second.Code != http.StatusForbidden {
		t.Fatalf("second status = %d, want 403: %s", second.Code, second.Body.String())
	}
	if repository.listRoleCalls != 2 {
		t.Fatalf("ListRolesForUser calls = %d, want 2", repository.listRoleCalls)
	}
}
