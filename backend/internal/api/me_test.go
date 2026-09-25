package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type currentUserRepositoryStub struct {
	user        store.User
	roles       []string
	updatedName string
	requestedID uuid.UUID
}

func (s *currentUserRepositoryStub) GetUserByID(_ context.Context, id uuid.UUID) (store.User, error) {
	s.requestedID = id
	return s.user, nil
}

func (s *currentUserRepositoryStub) ListRolesForUser(_ context.Context, id uuid.UUID) ([]string, error) {
	s.requestedID = id
	return append([]string(nil), s.roles...), nil
}

func (s *currentUserRepositoryStub) UpdateDisplayName(_ context.Context, _ uuid.UUID, displayName string) (store.User, error) {
	s.updatedName = displayName
	s.user.DisplayName = displayName
	return s.user, nil
}

func testCurrentUser() store.User {
	return store.User{
		ID:           uuid.MustParse("018f90f2-9d08-7d6c-9b11-0e77d842d66d"),
		Email:        "ada@example.com",
		PasswordHash: "secret-argon2-hash",
		DisplayName:  "Ada",
		Status:       "active",
		MFAEnabled:   true,
		CreatedAt:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
}

func authenticatedCurrentUserRequest(t *testing.T, method string, body string, userID uuid.UUID) *http.Request {
	t.Helper()
	raw, err := apiTestService(t).Issue(userID.String(), []string{"admin"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	request := httptest.NewRequest(method, "/api/v1/me", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+raw)
	return request
}

func TestRF008_MeDevuelveElUsuarioAutenticado(t *testing.T) {
	user := testCurrentUser()
	repository := &currentUserRepositoryStub{user: user, roles: []string{"user"}}
	server := NewServer(nil, "test", nil)
	server.SetTokenService(apiTestService(t))
	server.SetCurrentUserRepository(repository)

	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, authenticatedCurrentUserRequest(t, http.MethodGet, "", user.ID))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}

	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["id"] != user.ID.String() || body["displayName"] != "Ada" {
		t.Fatalf("body = %#v", body)
	}
	if repository.requestedID != user.ID {
		t.Fatalf("repository user ID = %s, want token subject %s", repository.requestedID, user.ID)
	}
	roles, ok := body["roles"].([]any)
	if !ok || len(roles) != 1 || roles[0] != "user" {
		t.Fatalf("roles = %#v, want roles from repository", body["roles"])
	}
}

func TestRF008_SinTokenDevuelve401(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetTokenService(apiTestService(t))
	server.SetCurrentUserRepository(&currentUserRepositoryStub{user: testCurrentUser()})

	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", response.Code, response.Body.String())
	}
}

func TestRF008_ActualizaElNombre(t *testing.T) {
	user := testCurrentUser()
	repository := &currentUserRepositoryStub{user: user, roles: []string{"user"}}
	server := NewServer(nil, "test", nil)
	server.SetTokenService(apiTestService(t))
	server.SetCurrentUserRepository(repository)

	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, authenticatedCurrentUserRequest(t, http.MethodPatch, "{\"displayName\":\"Grace Hopper\"}", user.ID))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	if repository.updatedName != "Grace Hopper" {
		t.Fatalf("updated display name = %q", repository.updatedName)
	}
}

func TestRF008_RespuestaNoIncluyePasswordHash(t *testing.T) {
	user := testCurrentUser()
	server := NewServer(nil, "test", nil)
	server.SetTokenService(apiTestService(t))
	server.SetCurrentUserRepository(&currentUserRepositoryStub{user: user, roles: []string{"user"}})

	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, authenticatedCurrentUserRequest(t, http.MethodGet, "", user.ID))
	body := strings.ToLower(response.Body.String())
	if strings.Contains(body, "passwordhash") || strings.Contains(body, "password_hash") || strings.Contains(body, user.PasswordHash) {
		t.Fatalf("response exposes password hash: %s", response.Body.String())
	}
}

func TestRF008_NombreInvalidoDevuelve400(t *testing.T) {
	user := testCurrentUser()
	for name, body := range map[string]string{
		"empty":    "{\"displayName\":\"\"}",
		"too long": "{\"displayName\":\"" + strings.Repeat("a", 101) + "\"}",
	} {
		t.Run(name, func(t *testing.T) {
			server := NewServer(nil, "test", nil)
			server.SetTokenService(apiTestService(t))
			server.SetCurrentUserRepository(&currentUserRepositoryStub{user: user, roles: []string{"user"}})

			response := httptest.NewRecorder()
			server.Routes().ServeHTTP(response, authenticatedCurrentUserRequest(t, http.MethodPatch, body, user.ID))
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", response.Code, response.Body.String())
			}
		})
	}
}
