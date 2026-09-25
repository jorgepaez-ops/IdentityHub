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
	"github.com/jorgepaez/identity-hub/internal/auth/registration"
)

type registrationStub struct {
	result registration.Result
	err    error
}

func (s registrationStub) Register(context.Context, registration.Input) (registration.Result, error) {
	return s.result, s.err
}

func TestRF001_RegistroDevuelve201YCuentaPendiente(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetRegistrationService(registrationStub{result: registration.Result{ID: uuid.New(), Email: "ada@example.com", Status: "pending_verification"}})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"ada@example.com","password":"correct horse battery","displayName":"Ada"}`))
	server.Register(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", response.Code, response.Body.String())
	}
	var body RegisterResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.Status != UserStatus("pending_verification") || string(body.Email) != "ada@example.com" {
		t.Fatalf("response = %+v", body)
	}
}

func TestRF001_ContrasenaCortaDevuelve400ConCampo(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetRegistrationService(registrationStub{err: &registration.InvalidInputError{Field: "password", Detail: "must contain 12 to 128 characters"}})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"ada@example.com","password":"short","displayName":"Ada"}`))
	server.Register(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
	var body struct {
		Errors []struct {
			Field string `json:"field"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Errors) != 1 || body.Errors[0].Field != "password" {
		t.Fatalf("errors = %+v", body.Errors)
	}
}

func TestRF001_CorreoDuplicadoDevuelve409SinRevelarExistencia(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetRegistrationService(registrationStub{err: registration.ErrEmailExists})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"ada@example.com","password":"correct horse battery","displayName":"Ada"}`))
	server.Register(response, request)
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409", response.Code)
	}
	if strings.Contains(strings.ToLower(response.Body.String()), "existe") {
		t.Fatalf("response reveals existence: %s", response.Body.String())
	}
}

func TestRF001_FalloDelBrokerRevierteYDevuelve503(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetRegistrationService(registrationStub{err: fmtWrap(registration.ErrPublish)})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"ada@example.com","password":"correct horse battery","displayName":"Ada"}`))
	server.Register(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
}
func fmtWrap(err error) error { return errors.Join(err, errors.New("broker unavailable")) }
