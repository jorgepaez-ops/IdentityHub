//go:build integration

package auditlog_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/api"
	"github.com/jorgepaez/identity-hub/internal/auth/auditlog"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRF011_UnLoginFallidoCreaUnaFila(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	passwordHash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	failedUser, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "failed-login@example.test", PasswordHash: passwordHash, DisplayName: "Failed Login"})
	if err != nil {
		t.Fatalf("CreateUser failed-login: %v", err)
	}
	adminUser, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "audit-admin@example.test", PasswordHash: passwordHash, DisplayName: "Audit Admin"})
	if err != nil {
		t.Fatalf("CreateUser admin: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, failedUser.ID); err != nil {
		t.Fatalf("activate failed-login user: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, adminUser.ID); err != nil {
		t.Fatalf("activate admin user: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id, granted_by) SELECT $1, id, $1 FROM roles WHERE name = 'admin'`, adminUser.ID); err != nil {
		t.Fatalf("grant admin: %v", err)
	}

	tokens, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatalf("token.New: %v", err)
	}
	if _, err := login.New(repository, tokens, time.Hour).Login(ctx, login.Input{Email: failedUser.Email, Password: "wrong password"}); !errors.Is(err, login.ErrInvalidCredentials) {
		t.Fatalf("failed login error=%v, want invalid credentials", err)
	}

	server := api.NewServer(nil, "test", nil)
	server.SetTokenService(tokens)
	server.SetCurrentUserRepository(repository)
	server.SetAuditLogService(auditlog.New(repository))
	raw, err := tokens.Issue(adminUser.ID.String(), []string{"admin"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-log?action=login_failed&actorId="+failedUser.ID.String(), nil)
	request.Header.Set("Authorization", "Bearer "+raw)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Count(response.Body.String(), `"action":"login_failed"`) != 1 || !strings.Contains(response.Body.String(), failedUser.ID.String()) {
		t.Fatalf("audit response=%s, want exactly one failed login for the user", response.Body.String())
	}
}

func TestRF011_FiltraPorAccionYActor(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	passwordHash, err := password.Hash("correct horse battery")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	actor, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "audit-actor@example.test", PasswordHash: passwordHash, DisplayName: "Audit Actor"})
	if err != nil {
		t.Fatalf("CreateUser actor: %v", err)
	}
	otherActor, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "audit-other@example.test", PasswordHash: passwordHash, DisplayName: "Other Actor"})
	if err != nil {
		t.Fatalf("CreateUser other actor: %v", err)
	}
	adminUser, err := repository.CreateUser(ctx, store.CreateUserParams{Email: "filter-admin@example.test", PasswordHash: passwordHash, DisplayName: "Filter Admin"})
	if err != nil {
		t.Fatalf("CreateUser admin: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, adminUser.ID); err != nil {
		t.Fatalf("activate admin: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO user_roles (user_id, role_id, granted_by) SELECT $1, id, $1 FROM roles WHERE name = 'admin'`, adminUser.ID); err != nil {
		t.Fatalf("grant admin: %v", err)
	}
	for _, event := range []store.InsertAuditEventParams{
		{ActorUserID: &actor.ID, Action: "login_failed", Metadata: []byte(`{}`)},
		{ActorUserID: &actor.ID, Action: "login_succeeded", Metadata: []byte(`{}`)},
		{ActorUserID: &otherActor.ID, Action: "login_failed", Metadata: []byte(`{}`)},
	} {
		if _, err := repository.InsertAuditEvent(ctx, event); err != nil {
			t.Fatalf("InsertAuditEvent: %v", err)
		}
	}
	tokens, err := token.New(make([]byte, 32), "https://issuer.test", "identity-hub", time.Now)
	if err != nil {
		t.Fatalf("token.New: %v", err)
	}
	server := api.NewServer(nil, "test", nil)
	server.SetTokenService(tokens)
	server.SetCurrentUserRepository(repository)
	server.SetAuditLogService(auditlog.New(repository))
	raw, err := tokens.Issue(adminUser.ID.String(), []string{"admin"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit-log?action=login_failed&actorId="+actor.ID.String(), nil)
	request.Header.Set("Authorization", "Bearer "+raw)
	response := httptest.NewRecorder()
	server.Routes().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	if strings.Count(response.Body.String(), `"action":"login_failed"`) != 1 || !strings.Contains(response.Body.String(), actor.ID.String()) || strings.Contains(response.Body.String(), otherActor.ID.String()) || strings.Contains(response.Body.String(), `"action":"login_succeeded"`) {
		t.Fatalf("filtered response=%s", response.Body.String())
	}
}

// TestRF011_ElRolDeLaAplicacionNoPuedeModificarLaFila is a regression check for
// the new read path added in T19: it must not have granted identity_app (the
// role the running API connects as, T9/000002) any write privilege on
// audit_log. Same technique as
// TestRF011_IdentityAppNoTieneUpdateNiDeleteSobreAuditLog in
// internal/audit/audit_integration_test.go.
func TestRF011_ElRolDeLaAplicacionNoPuedeModificarLaFila(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)

	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO audit_log (action) VALUES ('login_failed') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("insert audit event: %v", err)
	}

	if _, err := pool.Exec(ctx, `SET ROLE identity_app`); err != nil {
		t.Fatalf("set application role: %v", err)
	}
	_, updateErr := pool.Exec(ctx, `UPDATE audit_log SET action = 'tampered' WHERE id = $1`, id)
	_, deleteErr := pool.Exec(ctx, `DELETE FROM audit_log WHERE id = $1`, id)
	if _, err := pool.Exec(ctx, `RESET ROLE`); err != nil {
		t.Fatalf("reset application role: %v", err)
	}
	for operation, err := range map[string]error{"UPDATE": updateErr, "DELETE": deleteErr} {
		if err == nil || !strings.Contains(err.Error(), "permission denied") {
			t.Errorf("%s error = %v, want permission denied", operation, err)
		}
	}
}
