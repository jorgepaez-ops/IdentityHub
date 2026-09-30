package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/session"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

type sessionStub struct {
	items       []session.Session
	listErr     error
	revokeErr   error
	revokedUser uuid.UUID
	revokedID   uuid.UUID
}

func (s *sessionStub) List(context.Context, uuid.UUID) ([]session.Session, error) {
	return s.items, s.listErr
}
func (s *sessionStub) Revoke(_ context.Context, userID, familyID uuid.UUID) error {
	s.revokedUser, s.revokedID = userID, familyID
	return s.revokeErr
}

func TestRF016_ListarSesionesDelTitularMarcaLaActual(t *testing.T) {
	userID, currentID, otherID := uuid.New(), uuid.New(), uuid.New()
	now := time.Date(2026, 9, 28, 14, 0, 0, 0, time.UTC)
	service := &sessionStub{items: []session.Session{{ID: currentID, IP: nil, UserAgent: "current", CreatedAt: now.Add(-time.Hour), LastUsedAt: now}, {ID: otherID, UserAgent: "other", CreatedAt: now.Add(-2 * time.Hour), LastUsedAt: now.Add(-time.Hour)}}}
	server := NewServer(nil, "test", nil)
	server.SetSessionService(service)
	server.SetTokenService(testSessionToken(t))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me/sessions", nil)
	request.Header.Set("Authorization", "Bearer "+testSessionBearer(t, server.tokens, userID, currentID))
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var got []Session
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !got[0].Current || got[1].Current || got[0].Ip != "" {
		t.Fatalf("sessions=%+v", got)
	}
}

func TestRF016_RevocarSesionPropiaDevuelve204(t *testing.T) {
	userID, familyID := uuid.New(), uuid.New()
	service := &sessionStub{}
	server := NewServer(nil, "test", nil)
	server.SetSessionService(service)
	server.SetTokenService(testSessionToken(t))
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/me/sessions/"+familyID.String(), nil)
	request.Header.Set("Authorization", "Bearer "+testSessionBearer(t, server.tokens, userID, familyID))
	response := httptest.NewRecorder()

	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || service.revokedUser != userID || service.revokedID != familyID {
		t.Fatalf("status=%d revokedUser=%s revokedID=%s", response.Code, service.revokedUser, service.revokedID)
	}
}

func TestRF016_ErroresDeListadoDeSesiones(t *testing.T) {
	userID, familyID := uuid.New(), uuid.New()
	for _, tt := range []struct {
		name    string
		service sessionManager
		want    int
	}{
		{name: "service unavailable", want: http.StatusServiceUnavailable},
		{name: "repository failure", service: &sessionStub{listErr: errors.New("database unavailable")}, want: http.StatusInternalServerError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(nil, "test", nil)
			server.SetSessionService(tt.service)
			server.SetTokenService(testSessionToken(t))
			request := httptest.NewRequest(http.MethodGet, "/api/v1/me/sessions", nil)
			request.Header.Set("Authorization", "Bearer "+testSessionBearer(t, server.tokens, userID, familyID))
			response := httptest.NewRecorder()

			server.Routes().ServeHTTP(response, request)
			if response.Code != tt.want {
				t.Fatalf("status=%d body=%s, want %d", response.Code, response.Body.String(), tt.want)
			}
		})
	}
}

func TestRF016_ErroresDeRevocacionDeSesiones(t *testing.T) {
	userID, familyID := uuid.New(), uuid.New()
	for _, tt := range []struct {
		name    string
		service sessionManager
		want    int
	}{
		{name: "missing session", service: &sessionStub{revokeErr: session.ErrSessionNotFound}, want: http.StatusNotFound},
		{name: "repository failure", service: &sessionStub{revokeErr: errors.New("database unavailable")}, want: http.StatusInternalServerError},
		{name: "service unavailable", want: http.StatusServiceUnavailable},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(nil, "test", nil)
			server.SetSessionService(tt.service)
			server.SetTokenService(testSessionToken(t))
			request := httptest.NewRequest(http.MethodDelete, "/api/v1/me/sessions/"+familyID.String(), nil)
			request.Header.Set("Authorization", "Bearer "+testSessionBearer(t, server.tokens, userID, familyID))
			response := httptest.NewRecorder()

			server.Routes().ServeHTTP(response, request)
			if response.Code != tt.want {
				t.Fatalf("status=%d body=%s, want %d", response.Code, response.Body.String(), tt.want)
			}
		})
	}
}

func testSessionToken(t *testing.T) *token.Service {
	t.Helper()
	service, err := token.New(make([]byte, 32), "identity-hub", "identity-hub-api", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return service
}
func testSessionBearer(t *testing.T, signer *token.Service, userID, familyID uuid.UUID) string {
	t.Helper()
	raw, err := signer.IssueForSession(userID.String(), []string{"user"}, familyID.String())
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
