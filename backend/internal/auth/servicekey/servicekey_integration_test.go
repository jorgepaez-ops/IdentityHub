//go:build integration

package servicekey_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jorgepaez/identity-hub/internal/api"
	"github.com/jorgepaez/identity-hub/internal/auth/auditlog"
	"github.com/jorgepaez/identity-hub/internal/auth/servicekey"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

type harness struct {
	t      *testing.T
	pool   *pgxpool.Pool
	server *api.Server
	tokens *token.Service
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	tokens, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatalf("token.New: %v", err)
	}
	server := api.NewServer(nil, "test", nil)
	server.SetTokenService(tokens)
	server.SetCurrentUserRepository(repository)
	server.SetAuditLogService(auditlog.New(repository))
	server.SetServiceKeyService(servicekey.New(repository.ServiceKeys()))
	return &harness{t: t, pool: pool, server: server, tokens: tokens}
}

// user creates an active account; it holds the admin role when admin is true.
func (h *harness) user(admin bool) (uuid.UUID, string) {
	h.t.Helper()
	ctx := context.Background()
	id := uuid.New()
	hash := "$argon2id$v=19$m=65536,t=3,p=2$c29tZXNhbHQ$c29tZWhhc2hzb21laGFzaHNvbWVoYXNoc29tZWhhc2g"
	if _, err := h.pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, display_name, status) VALUES ($1, $2, $3, 'Test', 'active')`, id, id.String()+"@example.test", hash); err != nil {
		h.t.Fatalf("insert user: %v", err)
	}
	roles := []string{"user"}
	if admin {
		roles = []string{"admin"}
		if _, err := h.pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id, granted_by) SELECT $1, id, $1 FROM roles WHERE name = 'admin'`, id); err != nil {
			h.t.Fatalf("grant admin: %v", err)
		}
	}
	raw, err := h.tokens.Issue(id.String(), roles)
	if err != nil {
		h.t.Fatalf("Issue: %v", err)
	}
	return id, raw
}

func (h *harness) do(method, path, body, bearer string) *httptest.ResponseRecorder {
	h.t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		request.Header.Set("Authorization", "Bearer "+bearer)
	}
	request.RemoteAddr = "203.0.113.30:4000"
	response := httptest.NewRecorder()
	h.server.Routes().ServeHTTP(response, request)
	return response
}

type issued struct {
	ID     uuid.UUID `json:"id"`
	Key    string    `json:"key"`
	Prefix string    `json:"prefix"`
}

func (h *harness) issue(adminToken string) issued {
	h.t.Helper()
	response := h.do(http.MethodPost, "/api/v1/admin/service-keys", `{"name":"SIEM","scopes":["audit-log:read"],"expiresInDays":30}`, adminToken)
	if response.Code != http.StatusCreated {
		h.t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
	}
	var key issued
	if err := json.Unmarshal(response.Body.Bytes(), &key); err != nil {
		h.t.Fatal(err)
	}
	return key
}

func secretOf(key string) string {
	parts := strings.SplitN(strings.TrimPrefix(key, "ihk_"), "_", 2)
	return parts[1]
}

func TestRF018_Integration_CrearDevuelveElSecretoUnaVezYGuardaSoloElHash(t *testing.T) {
	h := newHarness(t)
	_, adminToken := h.user(true)
	key := h.issue(adminToken)
	secret := secretOf(key.Key)

	var hash []byte
	var rowText string
	if err := h.pool.QueryRow(context.Background(), `SELECT secret_hash, s::text FROM service_keys s WHERE id = $1`, key.ID).Scan(&hash, &rowText); err != nil {
		t.Fatalf("select key: %v", err)
	}
	if want := sha256.Sum256([]byte(secret)); string(hash) != string(want[:]) {
		t.Fatal("the stored hash is not the SHA-256 of the secret")
	}
	if strings.Contains(rowText, secret) {
		t.Fatal("the database row contains the secret")
	}

	list := h.do(http.MethodGet, "/api/v1/admin/service-keys", "", adminToken)
	if list.Code != http.StatusOK || strings.Contains(list.Body.String(), secret) || strings.Contains(strings.ToLower(list.Body.String()), "hash") || strings.Contains(list.Body.String(), `"key"`) {
		t.Fatalf("list must expose metadata only: status=%d body=%s", list.Code, list.Body.String())
	}
	if !strings.Contains(list.Body.String(), key.Prefix) {
		t.Fatalf("list lacks the prefix: %s", list.Body.String())
	}
}

func TestRF018_Integration_LaClaveListaElAuditLogYRevocarlaLaInutiliza(t *testing.T) {
	h := newHarness(t)
	_, adminToken := h.user(true)
	key := h.issue(adminToken)

	if response := h.do(http.MethodGet, "/api/v1/admin/audit-log", "", key.Key); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "service_key_created") {
		t.Fatalf("key on audit log status=%d body=%s", response.Code, response.Body.String())
	}
	var lastUsed *time.Time
	if err := h.pool.QueryRow(context.Background(), `SELECT last_used_at FROM service_keys WHERE id = $1`, key.ID).Scan(&lastUsed); err != nil || lastUsed == nil {
		t.Fatalf("last_used_at must be set after use: %v %v", lastUsed, err)
	}

	for range 2 {
		if response := h.do(http.MethodDelete, "/api/v1/admin/service-keys/"+key.ID.String(), "", adminToken); response.Code != http.StatusNoContent {
			t.Fatalf("revoke status=%d body=%s", response.Code, response.Body.String())
		}
	}
	if response := h.do(http.MethodGet, "/api/v1/admin/audit-log", "", key.Key); response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked key status=%d, want 401", response.Code)
	}
	if response := h.do(http.MethodDelete, "/api/v1/admin/service-keys/"+uuid.NewString(), "", adminToken); response.Code != http.StatusNotFound {
		t.Fatalf("unknown key revoke status=%d, want 404", response.Code)
	}
}

func TestRF018_Integration_LaClaveNoAbreOtrasOperacionesNiGestionaClaves(t *testing.T) {
	h := newHarness(t)
	_, adminToken := h.user(true)
	key := h.issue(adminToken)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/v1/admin/users", ""},
		{http.MethodGet, "/api/v1/admin/applications", ""},
		{http.MethodGet, "/api/v1/admin/service-keys", ""},
		{http.MethodPost, "/api/v1/admin/service-keys", `{"name":"x","scopes":["audit-log:read"],"expiresInDays":3}`},
		{http.MethodDelete, "/api/v1/admin/service-keys/" + key.ID.String(), ""},
		{http.MethodGet, "/api/v1/me", ""},
	} {
		if response := h.do(tc.method, tc.path, tc.body, key.Key); response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s with a service key status=%d, want 401", tc.method, tc.path, response.Code)
		}
	}
	var revoked *time.Time
	if err := h.pool.QueryRow(context.Background(), `SELECT revoked_at FROM service_keys WHERE id = $1`, key.ID).Scan(&revoked); err != nil || revoked != nil {
		t.Fatalf("a key must not revoke itself: %v %v", revoked, err)
	}
}

func TestRF018_Integration_TodasLasClavesInvalidasRecibenLaMismaRespuestaYQuedanAuditadas(t *testing.T) {
	h := newHarness(t)
	_, adminToken := h.user(true)
	valid := h.issue(adminToken)
	expired := h.issue(adminToken)
	revoked := h.issue(adminToken)
	ctx := context.Background()
	if _, err := h.pool.Exec(ctx, `UPDATE service_keys SET created_at = now() - interval '2 days', expires_at = now() - interval '1 day' WHERE id = $1`, expired.ID); err != nil {
		t.Fatal(err)
	}
	if response := h.do(http.MethodDelete, "/api/v1/admin/service-keys/"+revoked.ID.String(), "", adminToken); response.Code != http.StatusNoContent {
		t.Fatalf("revoke status=%d", response.Code)
	}
	publicID := strings.TrimPrefix(valid.Prefix, "ihk_")
	cases := map[string]string{
		"wrong value": "ihk_" + publicID + "_" + strings.Repeat("A", 43),
		"unknown id":  "ihk_0000000000000000_" + secretOf(valid.Key),
		"expired":     expired.Key,
		"revoked":     revoked.Key,
		"malformed":   "ihk_garbage",
	}
	h.requireSameUnauthorizedResponse(t, cases)
	h.requireRejectedAuditReasons(t)
}

// requireSameUnauthorizedResponse sends each credential to the audit-log endpoint and checks all
// of them get a 401 with an identical body, so the response is no oracle for the failure kind.
func (h *harness) requireSameUnauthorizedResponse(t *testing.T, cases map[string]string) {
	t.Helper()
	var first string
	for name, raw := range cases {
		response := h.do(http.MethodGet, "/api/v1/admin/audit-log", "", raw)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status=%d body=%s", name, response.Code, response.Body.String())
		}
		if first == "" {
			first = response.Body.String()
		} else if response.Body.String() != first {
			t.Fatalf("%s: response differs from the others (oracle): %s", name, response.Body.String())
		}
	}
}

// requireRejectedAuditReasons checks the service_key_rejected audit rows: rejections tied to an
// existing key are audited once each; anonymous junk (unknown id, malformed) is not.
func (h *harness) requireRejectedAuditReasons(t *testing.T) {
	t.Helper()
	rows, err := h.pool.Query(context.Background(), `SELECT metadata->>'reason' FROM audit_log WHERE action = 'service_key_rejected' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	reasons := map[string]int{}
	for rows.Next() {
		var reason string
		if err := rows.Scan(&reason); err != nil {
			t.Fatal(err)
		}
		reasons[reason]++
	}
	for _, reason := range []string{"bad_secret", "expired", "revoked"} {
		if reasons[reason] != 1 {
			t.Fatalf("rejected audit reasons=%v, want one %q", reasons, reason)
		}
	}
	for _, reason := range []string{"unknown_key", "malformed"} {
		if reasons[reason] != 0 {
			t.Fatalf("rejected audit reasons=%v, anonymous %q must not be written to the audit log", reasons, reason)
		}
	}
}

func TestRF018_Integration_LaAuditoriaRegistraEmisionYRevocacionSinElSecreto(t *testing.T) {
	h := newHarness(t)
	adminID, adminToken := h.user(true)
	key := h.issue(adminToken)
	h.do(http.MethodDelete, "/api/v1/admin/service-keys/"+key.ID.String(), "", adminToken)
	h.do(http.MethodDelete, "/api/v1/admin/service-keys/"+key.ID.String(), "", adminToken)
	h.do(http.MethodGet, "/api/v1/admin/audit-log", "", key.Key)

	ctx := context.Background()
	counts := map[string]int{}
	rows, err := h.pool.Query(ctx, `SELECT action, actor_user_id, resource_id, ip::text, a::text FROM audit_log a WHERE action LIKE 'service_key_%' ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	secret := secretOf(key.Key)
	for rows.Next() {
		var action, text string
		var actor *uuid.UUID
		var resource, ip *string
		if err := rows.Scan(&action, &actor, &resource, &ip, &text); err != nil {
			t.Fatal(err)
		}
		counts[action]++
		if strings.Contains(text, secret) {
			t.Fatalf("audit event %s contains the secret", action)
		}
		if action != "service_key_rejected" && (actor == nil || *actor != adminID || resource == nil || *resource != key.ID.String() || ip == nil || !strings.HasPrefix(*ip, "203.0.113.30")) {
			t.Fatalf("event %s lacks actor/resource/ip: %v %v %v", action, actor, resource, ip)
		}
	}
	if counts["service_key_created"] != 1 || counts["service_key_revoked"] != 1 || counts["service_key_rejected"] != 1 {
		t.Fatalf("audit counts=%v, want created=1 revoked=1 (idempotent revoke) rejected=1", counts)
	}
}

func TestRF018_Integration_SoloUnAdminGestionaClaves(t *testing.T) {
	h := newHarness(t)
	_, adminToken := h.user(true)
	key := h.issue(adminToken)
	_, userToken := h.user(false)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/admin/service-keys", `{"name":"x","scopes":["audit-log:read"],"expiresInDays":3}`},
		{http.MethodGet, "/api/v1/admin/service-keys", ""},
		{http.MethodDelete, "/api/v1/admin/service-keys/" + key.ID.String(), ""},
	} {
		if response := h.do(tc.method, tc.path, tc.body, userToken); response.Code != http.StatusForbidden {
			t.Fatalf("%s %s as user status=%d, want 403", tc.method, tc.path, response.Code)
		}
		if response := h.do(tc.method, tc.path, tc.body, ""); response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s anonymous status=%d, want 401", tc.method, tc.path, response.Code)
		}
	}
	var count int
	if err := h.pool.QueryRow(context.Background(), `SELECT count(*) FROM service_keys WHERE revoked_at IS NULL`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("keys=%d err=%v, a non-admin must change nothing", count, err)
	}
}

func TestRF018_Integration_LaBaseRechazaDatosInvalidos(t *testing.T) {
	h := newHarness(t)
	ctx := context.Background()
	for name, query := range map[string]string{
		"public id format": `INSERT INTO service_keys (public_id, secret_hash, name, scopes, expires_at) VALUES ('XYZ', decode(repeat('ab', 32), 'hex'), 'k', ARRAY['audit-log:read'], now() + interval '1 day')`,
		"hash length":      `INSERT INTO service_keys (public_id, secret_hash, name, scopes, expires_at) VALUES ('0123456789abcdef', '\x00', 'k', ARRAY['audit-log:read'], now() + interval '1 day')`,
		"no scopes":        `INSERT INTO service_keys (public_id, secret_hash, name, scopes, expires_at) VALUES ('0123456789abcdef', decode(repeat('ab', 32), 'hex'), 'k', ARRAY[]::text[], now() + interval '1 day')`,
		"expires in past":  `INSERT INTO service_keys (public_id, secret_hash, name, scopes, expires_at) VALUES ('0123456789abcdef', decode(repeat('ab', 32), 'hex'), 'k', ARRAY['audit-log:read'], now() - interval '1 day')`,
		"missing expiry":   `INSERT INTO service_keys (public_id, secret_hash, name, scopes) VALUES ('0123456789abcdef', decode(repeat('ab', 32), 'hex'), 'k', ARRAY['audit-log:read'])`,
	} {
		if _, err := h.pool.Exec(ctx, query); err == nil {
			t.Fatalf("%s: the database accepted an invalid key", name)
		}
	}
}
