package api

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/auditlog"
)

func sampleExportEvents(actor uuid.UUID) []auditlog.Event {
	resourceType, resourceID, ip := "user", "42", "198.51.100.7"
	return []auditlog.Event{{
		ID: 11, ActorUserID: &actor, Action: "login_failed", ResourceType: &resourceType, ResourceID: &resourceID, IP: &ip,
		Metadata: map[string]any{"reason": "bad_password"}, CreatedAt: time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC),
	}}
}

func TestRF019_ExportarRequiereAdmin(t *testing.T) {
	actor := uuid.New()
	service := &auditLogServiceStub{}
	server := auditLogServer(t, actor, []string{"user"}, service)
	response := adminRequest(t, server, http.MethodGet, "/api/v1/admin/audit-log/export?format=csv", "", actor)
	if response.Code != http.StatusForbidden || service.exports != 0 {
		t.Fatalf("status=%d exports=%d body=%s, want 403 and no export", response.Code, service.exports, response.Body.String())
	}
}

func TestRF019_FormatoInvalidoEsProblemJSON(t *testing.T) {
	actor := uuid.New()
	service := &auditLogServiceStub{}
	server := auditLogServer(t, actor, []string{"admin"}, service)
	for _, path := range []string{"/api/v1/admin/audit-log/export?format=xml", "/api/v1/admin/audit-log/export"} {
		response := adminRequest(t, server, http.MethodGet, path, "", actor)
		if response.Code != http.StatusBadRequest || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/problem+json") {
			t.Fatalf("%s: status=%d type=%q body=%s", path, response.Code, response.Header().Get("Content-Type"), response.Body.String())
		}
	}
	if service.exports != 0 {
		t.Fatalf("exports=%d, want none", service.exports)
	}
}

func TestRF019_ExportaCSVConLosMismosFiltros(t *testing.T) {
	actor, matched := uuid.New(), uuid.New()
	service := &auditLogServiceStub{result: auditlog.ExportResult{Events: sampleExportEvents(matched)}}
	server := auditLogServer(t, actor, []string{"admin"}, service)
	response := adminRequest(t, server, http.MethodGet, "/api/v1/admin/audit-log/export?format=csv&action=login_failed&actorId="+matched.String()+"&since=2026-09-01T00:00:00Z", "", actor)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Fatalf("Content-Type=%q", got)
	}
	disposition := response.Header().Get("Content-Disposition")
	if !regexp.MustCompile(`^attachment; filename="audit-log-\d{8}T\d{6}Z\.csv"$`).MatchString(disposition) {
		t.Fatalf("Content-Disposition=%q", disposition)
	}
	if response.Header().Get("X-Export-Truncated") != "false" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers=%v", response.Header())
	}
	in := service.export
	if in.Action == nil || *in.Action != "login_failed" || in.ActorID == nil || *in.ActorID != matched || in.Since == nil || in.Format != "csv" || in.RequestedBy != actor {
		t.Fatalf("export input=%+v", in)
	}
	body := bytes.TrimPrefix(response.Body.Bytes(), []byte("\xEF\xBB\xBF"))
	rows, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
	if err != nil || len(rows) != 2 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	if strings.Join(rows[0], ",") != "id,created_at,actor_user_id,action,resource_type,resource_id,ip,user_agent,metadata" {
		t.Fatalf("header=%v", rows[0])
	}
	want := []string{"11", "2026-09-25T10:00:00Z", matched.String(), "login_failed", "user", "42", "198.51.100.7", "", `{"reason":"bad_password"}`}
	if strings.Join(rows[1], "|") != strings.Join(want, "|") {
		t.Fatalf("row=%v want %v", rows[1], want)
	}
}

func TestRF019_ExportaJSON(t *testing.T) {
	actor, matched := uuid.New(), uuid.New()
	service := &auditLogServiceStub{result: auditlog.ExportResult{Events: sampleExportEvents(matched), Truncated: true}}
	server := auditLogServer(t, actor, []string{"admin"}, service)
	response := adminRequest(t, server, http.MethodGet, "/api/v1/admin/audit-log/export?format=json", "", actor)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/json; charset=utf-8" {
		t.Fatalf("status=%d type=%q", response.Code, response.Header().Get("Content-Type"))
	}
	if !regexp.MustCompile(`filename="audit-log-\d{8}T\d{6}Z\.json"`).MatchString(response.Header().Get("Content-Disposition")) || response.Header().Get("X-Export-Truncated") != "true" {
		t.Fatalf("headers=%v", response.Header())
	}
	var items []AuditEvent
	if err := json.Unmarshal(response.Body.Bytes(), &items); err != nil || len(items) != 1 || items[0].Id != 11 || items[0].Action != "login_failed" {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}

func TestRF019_ExportJSONVacioEsArreglo(t *testing.T) {
	actor := uuid.New()
	server := auditLogServer(t, actor, []string{"admin"}, &auditLogServiceStub{})
	response := adminRequest(t, server, http.MethodGet, "/api/v1/admin/audit-log/export?format=json", "", actor)
	if strings.TrimSpace(response.Body.String()) != "[]" {
		t.Fatalf("body=%q, want []", response.Body.String())
	}
}

func TestRF019_CSVNeutralizaInyeccionDeFormulas(t *testing.T) {
	actor := uuid.New()
	for _, prefix := range []string{"=", "+", "-", "@", "\t", "\r"} {
		payload := prefix + `HYPERLINK("http://evil.test","x")`
		events := []auditlog.Event{{ID: 1, Action: payload, UserAgent: &payload, Metadata: map[string]any{}, CreatedAt: time.Now()}}
		server := auditLogServer(t, actor, []string{"admin"}, &auditLogServiceStub{result: auditlog.ExportResult{Events: events}})
		response := adminRequest(t, server, http.MethodGet, "/api/v1/admin/audit-log/export?format=csv", "", actor)
		body := bytes.TrimPrefix(response.Body.Bytes(), []byte("\xEF\xBB\xBF"))
		rows, err := csv.NewReader(bytes.NewReader(body)).ReadAll()
		if err != nil || len(rows) != 2 {
			t.Fatalf("prefix %q: rows=%v err=%v", prefix, rows, err)
		}
		// encoding/csv drops a bare CR when UseCRLF is on; the neutralizing quote is what matters.
		want := "'" + strings.ReplaceAll(payload, "\r", "")
		if rows[1][3] != want || rows[1][7] != want {
			t.Fatalf("prefix %q: action=%q userAgent=%q, want single-quote prefix", prefix, rows[1][3], rows[1][7])
		}
	}
}

func TestRF019_ExportaFallosDelServicioComoProblem(t *testing.T) {
	actor := uuid.New()
	server := auditLogServer(t, actor, []string{"admin"}, &auditLogServiceStub{err: http.ErrAbortHandler})
	response := adminRequest(t, server, http.MethodGet, "/api/v1/admin/audit-log/export?format=csv", "", actor)
	if response.Code != http.StatusInternalServerError || !strings.HasPrefix(response.Header().Get("Content-Type"), "application/problem+json") {
		t.Fatalf("status=%d type=%q", response.Code, response.Header().Get("Content-Type"))
	}
	if strings.Contains(response.Body.String(), "id,created_at") {
		t.Fatal("no export data may be written when the service fails")
	}
	if response := adminRequest(t, auditLogServer(t, actor, []string{"admin"}, nil), http.MethodGet, "/api/v1/admin/audit-log/export?format=csv", "", actor); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", response.Code)
	}
}
