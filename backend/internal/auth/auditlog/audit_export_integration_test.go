//go:build integration

package auditlog_test

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jorgepaez/identity-hub/internal/api"
	"github.com/jorgepaez/identity-hub/internal/auth/auditlog"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

type exportFixture struct {
	pool   *pgxpool.Pool
	repo   *store.Store
	server *api.Server
	admin  uuid.UUID
	bearer string
}

func newExportFixture(t *testing.T) exportFixture {
	t.Helper()
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	admin, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "export-admin@example.test", PasswordHash: hash, DisplayName: "Export Admin"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, admin.ID); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id, granted_by) SELECT $1, id, $1 FROM roles WHERE name = 'admin'`, admin.ID); err != nil {
		t.Fatalf("grant admin: %v", err)
	}
	tokens, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatalf("token.New: %v", err)
	}
	server := api.NewServer(nil, "test", nil)
	server.SetTokenService(tokens)
	server.SetCurrentUserRepository(repository)
	server.SetAuditLogService(auditlog.New(repository))
	raw, err := tokens.Issue(admin.ID.String(), []string{"admin"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	return exportFixture{pool: pool, repo: repository, server: server, admin: admin.ID, bearer: raw}
}

func (f exportFixture) get(path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodGet, path, nil)
	request.Header.Set("Authorization", "Bearer "+f.bearer)
	request.Header.Set("User-Agent", "export-test/1.0")
	response := httptest.NewRecorder()
	f.server.Routes().ServeHTTP(response, request)
	return response
}

func TestRF019_ExportaFiltradoYAuditaLaExportacionEnPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	f := newExportFixture(t)
	for _, event := range []store.InsertAuditEventParams{
		{Action: "login_failed", Metadata: []byte(`{"reason":"bad_password"}`)},
		{Action: "login_succeeded", Metadata: []byte(`{}`)},
		{Action: "login_failed", Metadata: []byte(`{}`)},
	} {
		if _, err := f.repo.InsertAuditEvent(ctx, event); err != nil {
			t.Fatalf("InsertAuditEvent: %v", err)
		}
	}

	csvResponse := f.get("/api/v1/admin/audit-log/export?format=csv&action=login_failed")
	if csvResponse.Code != http.StatusOK {
		t.Fatalf("csv status=%d body=%s", csvResponse.Code, csvResponse.Body.String())
	}
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(csvResponse.Body.Bytes(), []byte("\xEF\xBB\xBF")))).ReadAll()
	if err != nil || len(rows) != 3 {
		t.Fatalf("csv rows=%v err=%v, want header plus two login_failed rows", rows, err)
	}
	for _, row := range rows[1:] {
		if row[3] != "login_failed" {
			t.Fatalf("row %v is not a login_failed event", row)
		}
	}

	jsonResponse := f.get("/api/v1/admin/audit-log/export?format=json&action=login_succeeded")
	var items []api.AuditEvent
	if err := json.Unmarshal(jsonResponse.Body.Bytes(), &items); err != nil || len(items) != 1 || items[0].Action != "login_succeeded" {
		t.Fatalf("json items=%+v err=%v", items, err)
	}

	var count int
	var metadata []byte
	var actor uuid.UUID
	var userAgent string
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action = 'audit_log_exported'`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("export audit events=%d err=%v, want 2", count, err)
	}
	if err := f.pool.QueryRow(ctx, `SELECT actor_user_id, user_agent, metadata FROM audit_log WHERE action = 'audit_log_exported' ORDER BY id LIMIT 1`).Scan(&actor, &userAgent, &metadata); err != nil {
		t.Fatalf("read export event: %v", err)
	}
	var decoded struct {
		Format    string            `json:"format"`
		Rows      int               `json:"rows"`
		Truncated bool              `json:"truncated"`
		Filters   map[string]string `json:"filters"`
	}
	if err := json.Unmarshal(metadata, &decoded); err != nil {
		t.Fatalf("metadata %s: %v", metadata, err)
	}
	if actor != f.admin || userAgent != "export-test/1.0" || decoded.Format != "csv" || decoded.Rows != 2 || decoded.Truncated || decoded.Filters["action"] != "login_failed" {
		t.Fatalf("export event actor=%s ua=%q metadata=%s", actor, userAgent, metadata)
	}
}

func TestRF019_ExportAcotaA10000FilasEnPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	f := newExportFixture(t)
	if _, err := f.pool.Exec(ctx, `INSERT INTO audit_log (action, metadata) SELECT 'bulk_event', '{}'::jsonb FROM generate_series(1, $1::int)`, auditlog.MaxExportRows+5); err != nil {
		t.Fatalf("seed bulk events: %v", err)
	}
	response := f.get("/api/v1/admin/audit-log/export?format=json&action=bulk_event")
	if response.Code != http.StatusOK || response.Header().Get("X-Export-Truncated") != "true" {
		t.Fatalf("status=%d truncated=%q", response.Code, response.Header().Get("X-Export-Truncated"))
	}
	var items []api.AuditEvent
	if err := json.Unmarshal(response.Body.Bytes(), &items); err != nil || len(items) != auditlog.MaxExportRows {
		t.Fatalf("items=%d err=%v, want %d", len(items), err, auditlog.MaxExportRows)
	}
}

func TestRF019_ExportarRequiereAdminEnPostgres(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	f := newExportFixture(t)
	tokens, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatalf("token.New: %v", err)
	}
	hash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	plain, err := f.repo.CreateUser(context.Background(), store.CreateUserParams{Email: "export-plain@example.test", PasswordHash: hash, DisplayName: "Plain User"})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := f.pool.Exec(context.Background(), `UPDATE users SET status = 'active' WHERE id = $1`, plain.ID); err != nil {
		t.Fatalf("activate: %v", err)
	}
	f.bearer, err = tokens.Issue(plain.ID.String(), []string{"user"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if response := f.get("/api/v1/admin/audit-log/export?format=csv"); response.Code != http.StatusForbidden {
		t.Fatalf("status=%d, want 403", response.Code)
	}
	var count int
	if err := f.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_log WHERE action = 'audit_log_exported'`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("export events=%d err=%v, want none for a forbidden request", count, err)
	}
}
