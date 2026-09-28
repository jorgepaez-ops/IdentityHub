package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jorgepaez/identity-hub/internal/auth/passwordreset"
)

type passwordResetStub struct {
	requestedEmail, token, password string
	requestErr, confirmErr          error
}

func (s *passwordResetStub) Request(_ context.Context, email string) error {
	s.requestedEmail = email
	return s.requestErr
}
func (s *passwordResetStub) Confirm(_ context.Context, token, password string) error {
	s.token, s.password = token, password
	return s.confirmErr
}

func TestRF015_SolicitudDevuelveRespuestaNoEnumerable(t *testing.T) {
	known, unknown := &passwordResetStub{}, &passwordResetStub{}
	for _, tt := range []struct {
		name, email string
		service     *passwordResetStub
	}{{"existente", "ada@example.test", known}, {"ausente", "nadie@example.test", unknown}} {
		t.Run(tt.name, func(t *testing.T) {
			s := NewServer(nil, "test", nil)
			s.SetPasswordResetService(tt.service)
			r := httptest.NewRecorder()
			s.Routes().ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", strings.NewReader(`{"email":"`+tt.email+`"}`)))
			if r.Code != http.StatusAccepted {
				t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
			}
			if r.Body.String() != "" {
				t.Fatalf("body=%q want empty", r.Body.String())
			}
		})
	}
}
func TestRF015_ConfirmacionInvalidaDevuelve410(t *testing.T) {
	s := NewServer(nil, "test", nil)
	s.SetPasswordResetService(&passwordResetStub{confirmErr: passwordreset.ErrTokenInvalid})
	r := httptest.NewRecorder()
	s.Routes().ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", strings.NewReader(`{"token":"used","password":"correct horse battery"}`)))
	if r.Code != http.StatusGone {
		t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
	}
}

func TestRF015_SolicitudMantiene202CuandoFallaLaEntrega(t *testing.T) {
	s := NewServer(nil, "test", nil)
	s.SetPasswordResetService(&passwordResetStub{requestErr: errors.New("broker unavailable")})
	r := httptest.NewRecorder()
	s.Routes().ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/request", strings.NewReader(`{"email":"ada@example.test"}`)))
	if r.Code != http.StatusAccepted || r.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q, want 202 empty", r.Code, r.Body.String())
	}
}

func TestRF015_ConfirmacionCorrectaDevuelve204(t *testing.T) {
	s := NewServer(nil, "test", nil)
	s.SetPasswordResetService(&passwordResetStub{})
	r := httptest.NewRecorder()
	s.Routes().ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", strings.NewReader(`{"token":"valid-token","password":"correct horse battery"}`)))
	if r.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
	}
}

func TestRF015_ConfirmacionConTokenVacioDevuelve400(t *testing.T) {
	s := NewServer(nil, "test", nil)
	s.SetPasswordResetService(&passwordResetStub{confirmErr: passwordreset.ErrTokenRequired})
	r := httptest.NewRecorder()
	s.Routes().ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", strings.NewReader(`{"token":"","password":"correct horse battery"}`)))
	if r.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
	}
}

func TestRF015_ConfirmacionConContrasenaInvalidaDevuelve400(t *testing.T) {
	s := NewServer(nil, "test", nil)
	s.SetPasswordResetService(&passwordResetStub{confirmErr: &passwordreset.InvalidInputError{Field: "password", Detail: "must contain 12 to 128 characters"}})
	r := httptest.NewRecorder()
	s.Routes().ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", strings.NewReader(`{"token":"valid-token","password":"short"}`)))
	if r.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
	}
}

func TestRF015_ConfirmacionConErrorInesperadoDevuelve500(t *testing.T) {
	s := NewServer(nil, "test", nil)
	s.SetPasswordResetService(&passwordResetStub{confirmErr: errors.New("database unavailable")})
	r := httptest.NewRecorder()
	s.Routes().ServeHTTP(r, httptest.NewRequest(http.MethodPost, "/api/v1/auth/password-reset/confirm", strings.NewReader(`{"token":"valid-token","password":"correct horse battery"}`)))
	if r.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
	}
}
