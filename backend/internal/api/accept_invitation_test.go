package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jorgepaez/identity-hub/internal/auth/invitation"
)

type invitationServiceStub struct {
	input invitation.Input
	err   error
}

func (s *invitationServiceStub) Accept(_ context.Context, input invitation.Input) error {
	s.input = input
	return s.err
}

func TestRF002_AcceptInvitationDevuelve204(t *testing.T) {
	service := &invitationServiceStub{}
	server := NewServer(nil, "test", nil)
	server.SetInvitationAcceptanceService(service)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/invitations/accept", strings.NewReader(`{"token":"invitation-token","password":"correct horse battery"}`))

	server.AcceptInvitation(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204: %s", response.Code, response.Body.String())
	}
	if service.input.Token != "invitation-token" || service.input.Password != "correct horse battery" {
		t.Fatalf("service received input = %+v", service.input)
	}
}

func TestRF002_AcceptInvitationTokenReutilizadoDevuelve410(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetInvitationAcceptanceService(&invitationServiceStub{err: invitation.ErrTokenInvalid})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/invitations/accept", strings.NewReader(`{"token":"used-token","password":"correct horse battery"}`))

	server.AcceptInvitation(response, request)
	if response.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410", response.Code)
	}
}

func TestRF002_AcceptInvitationContrasenaCortaDevuelve400ConCampo(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetInvitationAcceptanceService(&invitationServiceStub{err: &invitation.InvalidInputError{Field: "password", Detail: "must contain 12 to 128 characters"}})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/invitations/accept", strings.NewReader(`{"token":"invitation-token","password":"short"}`))

	server.AcceptInvitation(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", response.Code, response.Body.String())
	}
}

func TestRF002_AcceptInvitationTokenVacioDevuelve400(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetInvitationAcceptanceService(&invitationServiceStub{err: invitation.ErrInvalidInput})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/invitations/accept", strings.NewReader(`{"token":"","password":"correct horse battery"}`))

	server.AcceptInvitation(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func TestRF002_AcceptInvitationErrorDelBrokerDevuelve503(t *testing.T) {
	server := NewServer(nil, "test", nil)
	server.SetInvitationAcceptanceService(&invitationServiceStub{err: errors.Join(invitation.ErrPublish, errors.New("broker unavailable"))})
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/invitations/accept", strings.NewReader(`{"token":"invitation-token","password":"correct horse battery"}`))

	server.AcceptInvitation(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", response.Code)
	}
}

func TestRF001_AutorregistroPublicoSigueDevolviendo404(t *testing.T) {
	server := NewServer(nil, "test", nil)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{}`))

	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (self-registration stays retired, D9)", response.Code)
	}
}
