package api

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/invitationresend"
	"github.com/jorgepaez/identity-hub/internal/store"
	"net/http"
	"testing"
)

type resendInvitationStub struct {
	input invitationresend.Input
	err   error
}

func (s *resendInvitationStub) Resend(_ context.Context, input invitationresend.Input) error {
	s.input = input
	return s.err
}
func TestRF001_ReenvioDeInvitacionRequiereAdmin(t *testing.T) {
	service := &resendInvitationStub{}
	server, actor := resendInvitationServer(t, []string{"user"}, service)
	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/users/"+uuid.New().String()+"/invitation", "", actor)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if service.input.UserID != uuid.Nil {
		t.Fatal("resend ran for non-admin")
	}
}
func TestRF001_ReenvioDeInvitacionActivaDevuelve409(t *testing.T) {
	service := &resendInvitationStub{err: invitationresend.ErrAccountNotPending}
	server, actor := resendInvitationServer(t, []string{"admin"}, service)
	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/users/"+uuid.New().String()+"/invitation", "", actor)
	if response.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRF001_ReenvioDeInvitacionOKDevuelve204(t *testing.T) {
	service := &resendInvitationStub{}
	server, actor := resendInvitationServer(t, []string{"admin"}, service)
	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/users/"+uuid.New().String()+"/invitation", "", actor)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRF001_ReenvioDeInvitacionUsuarioAusenteDevuelve404(t *testing.T) {
	service := &resendInvitationStub{err: invitationresend.ErrNotFound}
	server, actor := resendInvitationServer(t, []string{"admin"}, service)
	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/users/"+uuid.New().String()+"/invitation", "", actor)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRF001_ReenvioDeInvitacionFalloDePublicacionDevuelve503(t *testing.T) {
	service := &resendInvitationStub{err: invitationresend.ErrPublish}
	server, actor := resendInvitationServer(t, []string{"admin"}, service)
	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/users/"+uuid.New().String()+"/invitation", "", actor)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestRF001_ReenvioDeInvitacionErrorInesperadoDevuelve500(t *testing.T) {
	service := &resendInvitationStub{err: errors.New("unexpected failure")}
	server, actor := resendInvitationServer(t, []string{"admin"}, service)
	response := adminRequest(t, server, http.MethodPost, "/api/v1/admin/users/"+uuid.New().String()+"/invitation", "", actor)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
func resendInvitationServer(t *testing.T, roles []string, service invitationresend.Resender) (*Server, uuid.UUID) {
	t.Helper()
	actor := uuid.New()
	s := NewServer(nil, "test", nil)
	s.SetTokenService(apiTestService(t))
	s.SetCurrentUserRepository(&rbacRepositoryStub{user: store.User{ID: actor, Status: "active"}, roles: roles})
	s.SetInvitationResendService(service)
	return s, actor
}
