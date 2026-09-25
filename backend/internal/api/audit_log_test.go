package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/auditlog"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type auditLogServiceStub struct {
	input  auditlog.ListInput
	events []auditlog.Event
	err    error
}

func (s *auditLogServiceStub) List(ctx context.Context, input auditlog.ListInput) ([]auditlog.Event, error) {
	s.input = input
	return s.events, s.err
}

func auditLogServer(t *testing.T, actor uuid.UUID, roles []string, service auditlog.Reader) *Server {
	t.Helper()
	server := NewServer(nil, "test", nil)
	server.SetTokenService(apiTestService(t))
	server.SetCurrentUserRepository(&rbacRepositoryStub{user: store.User{ID: actor, Status: "active"}, roles: roles})
	server.SetAuditLogService(service)
	return server
}

func TestRF011_ListarRequiereAdmin(t *testing.T) {
	actor := uuid.New()
	server := auditLogServer(t, actor, []string{"user"}, &auditLogServiceStub{})
	response := adminRequest(t, server, http.MethodGet, "/api/v1/admin/audit-log", "", actor)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s, want 403", response.Code, response.Body.String())
	}
}

func TestRF011_FiltrosSePasanAlServicio(t *testing.T) {
	actor := uuid.New()
	matchedActor := uuid.New()
	since := time.Date(2026, 9, 25, 0, 0, 0, 0, time.UTC)
	service := &auditLogServiceStub{events: []auditlog.Event{{
		ID:          11,
		ActorUserID: &matchedActor,
		Action:      "login_failed",
		Metadata:    map[string]any{},
		CreatedAt:   since,
	}}}
	server := auditLogServer(t, actor, []string{"admin"}, service)
	response := adminRequest(t, server, http.MethodGet, "/api/v1/admin/audit-log?action=login_failed&actorId="+matchedActor.String(), "", actor)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if service.input.Action == nil || *service.input.Action != "login_failed" || service.input.ActorID == nil || *service.input.ActorID != matchedActor {
		t.Fatalf("filters=%+v, want action and actor", service.input)
	}
	var page AuditLogPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].Id != 11 || page.Items[0].Action != "login_failed" {
		t.Fatalf("items=%+v", page.Items)
	}
}

func TestRF011_PaginacionUsaCursorDecimalDelUltimoID(t *testing.T) {
	actor := uuid.New()
	service := &auditLogServiceStub{events: []auditlog.Event{{ID: 20, Action: "login_failed", Metadata: map[string]any{}, CreatedAt: time.Now()}, {ID: 19, Action: "login_failed", Metadata: map[string]any{}, CreatedAt: time.Now()}}}
	server := auditLogServer(t, actor, []string{"admin"}, service)
	response := adminRequest(t, server, http.MethodGet, "/api/v1/admin/audit-log?limit=1", "", actor)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var page AuditLogPage
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.NextCursor == nil || *page.NextCursor != "20" {
		t.Fatalf("page=%+v", page)
	}
}
