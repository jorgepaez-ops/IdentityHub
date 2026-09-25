package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jorgepaez/identity-hub/internal/auth/verification"
)

type verificationStub struct{ err error }

func (s verificationStub) Verify(context.Context, verification.Input) error { return s.err }

func TestRF002_VerificarActivaLaCuenta(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetEmailVerificationService(verificationStub{})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", strings.NewReader(`{"token":"verification-token"}`))

	server.VerifyEmail(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", response.Code, response.Body.String())
	}
}

func TestRF002_EnlaceDeUnSoloUso(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetEmailVerificationService(verificationStub{err: verification.ErrTokenInvalid})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", strings.NewReader(`{"token":"used-token"}`))

	server.VerifyEmail(response, request)
	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", response.Code)
	}
}

func TestRF002_TokenExpiradoDevuelve410(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetEmailVerificationService(verificationStub{err: verification.ErrTokenInvalid})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", strings.NewReader(`{"token":"expired-token"}`))

	server.VerifyEmail(response, request)
	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", response.Code)
	}
}

func TestRF002_TokenVacioDevuelve400(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetEmailVerificationService(verificationStub{err: verification.ErrInvalidInput})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", strings.NewReader(`{"token":""}`))

	server.VerifyEmail(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func TestRF002_ErrorDelBrokerDevuelve503(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetEmailVerificationService(verificationStub{err: errors.Join(verification.ErrPublish, errors.New("broker unavailable"))})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", strings.NewReader(`{"token":"verification-token"}`))

	server.VerifyEmail(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
}
