package api

import (
	"context"
	"encoding/json"
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
	revokeErr   error
	revokedUser uuid.UUID
	revokedID   uuid.UUID
}

func (s *sessionStub) List(context.Context, uuid.UUID) ([]session.Session, error) {
	return s.items, nil
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
	server.SetTokenService(testSessionToken(t, userID, currentID))

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/v1/me/sessions", nil)
	request.Header.Set("Authorization", "Bearer "+testSessionBearer(t, server.tokens, userID, currentID))
	server.ListSessions(response, request)
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
	server.SetTokenService(testSessionToken(t, userID, familyID))
	request := httptest.NewRequest(http.MethodDelete, "/api/v1/me/sessions/"+familyID.String(), nil)
	request.Header.Set("Authorization", "Bearer "+testSessionBearer(t, server.tokens, userID, familyID))
	response := httptest.NewRecorder()

	server.RevokeSession(response, request, SessionId(familyID))
	if response.Code != http.StatusNoContent || service.revokedUser != userID || service.revokedID != familyID {
		t.Fatalf("status=%d revokedUser=%s revokedID=%s", response.Code, service.revokedUser, service.revokedID)
	}
}

func testSessionToken(t *testing.T, userID, familyID uuid.UUID) *token.Service {
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
