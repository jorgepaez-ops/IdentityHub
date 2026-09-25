package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

type loginStub struct {
	result login.Result
	err    error
}

func (s loginStub) Login(context.Context, login.Input) (login.Result, error) { return s.result, s.err }

func TestRF003_LoginCorrectoDevuelveParDeTokens(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetLoginService(loginStub{result: login.Result{AccessToken: "access-token", TokenType: "Bearer", ExpiresIn: 900, RefreshToken: "opaque-refresh"}}, 720*time.Hour)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"correct horse battery"}`))

	server.Login(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["accessToken"] != "access-token" || body["refreshToken"] != nil {
		t.Fatalf("body = %#v", body)
	}
	cookie := response.Result().Cookies()[0]
	if !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/api/v1/auth" {
		t.Fatalf("cookie = %+v", cookie)
	}
}

func TestRF003_PasswordIncorrectoDevuelve401Generico(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetLoginService(loginStub{err: login.ErrInvalidCredentials}, time.Hour)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"wrong"}`))
	server.Login(response, request)
	if response.Code != http.StatusUnauthorized || strings.Contains(response.Body.String(), "ada@example.com") {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
}

func TestRF003_CuentaConMFARechazadaConErrorClaro(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetLoginService(loginStub{err: errors.Join(login.ErrMFAUnavailable, errors.New("internal context"))}, time.Hour)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"correct horse battery"}`))
	server.Login(response, request)
	if response.Code != http.StatusNotImplemented || len(response.Result().Cookies()) != 0 || !strings.Contains(response.Body.String(), "MFA is not supported") {
		t.Fatalf("response = %d cookies=%v %s", response.Code, response.Result().Cookies(), response.Body.String())
	}
}

type realLoginRepositoryStub struct {
	user      login.User
	lookupErr error
	audits    []login.AuditEvent
}

func (s *realLoginRepositoryStub) WithinLoginTransaction(_ context.Context, fn func(login.Writer) error) error {
	return fn(s)
}
func (s *realLoginRepositoryStub) GetLoginUserByEmail(context.Context, string) (login.User, error) {
	return s.user, s.lookupErr
}
func (*realLoginRepositoryStub) ListRolesForUser(context.Context, uuid.UUID) ([]string, error) {
	return []string{"user"}, nil
}
func (*realLoginRepositoryStub) UpdateLoginSuccess(context.Context, uuid.UUID, string) error {
	return nil
}
func (*realLoginRepositoryStub) CreateRefreshToken(context.Context, login.RefreshToken) error {
	return nil
}
func (s *realLoginRepositoryStub) InsertAuditEvent(_ context.Context, event login.AuditEvent) error {
	s.audits = append(s.audits, event)
	return nil
}

func TestRF003_AM004EmailInexistenteYPasswordIncorrectoDevuelvenElMismoMensaje(t *testing.T) {
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		repo *realLoginRepositoryStub
	}{
		{name: "unknown email", repo: &realLoginRepositoryStub{lookupErr: pgx.ErrNoRows}},
		{name: "wrong password", repo: &realLoginRepositoryStub{user: login.User{ID: uuid.New(), PasswordHash: hash, Status: login.StatusActive}}},
	}
	var responseBodies []string
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			server := NewServer(nil, "test", nil)
			server.SetLoginService(login.New(testCase.repo, signer, time.Hour), time.Hour)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"ada@example.com","password":"wrong password"}`))
			server.Login(response, request)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401: %s", response.Code, response.Body.String())
			}
			responseBodies = append(responseBodies, response.Body.String())
		})
	}
	if responseBodies[0] != responseBodies[1] {
		t.Fatalf("responses disclose account existence: unknown=%s wrong-password=%s", responseBodies[0], responseBodies[1])
	}
}
