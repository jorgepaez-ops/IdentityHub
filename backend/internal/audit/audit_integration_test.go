//go:build integration

package audit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRF011_IdentityAppNoTieneUpdateNiDeleteSobreAuditLog(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}

	pool := testdb.New(t)
	ctx := context.Background()
	var canUpdate, canDelete bool
	if err := pool.QueryRow(ctx, `SELECT has_table_privilege('identity_app', 'audit_log', 'UPDATE'), has_table_privilege('identity_app', 'audit_log', 'DELETE')`).Scan(&canUpdate, &canDelete); err != nil {
		t.Fatalf("read audit permissions: %v", err)
	}
	if canUpdate || canDelete {
		t.Fatalf("identity_app privileges update=%t delete=%t, want both false", canUpdate, canDelete)
	}

	var id int64
	if err := pool.QueryRow(ctx, `INSERT INTO audit_log (action) VALUES ('login_failed') RETURNING id`).Scan(&id); err != nil {
		t.Fatalf("insert audit event: %v", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE audit_log DISABLE TRIGGER audit_log_no_update`); err != nil {
		t.Fatalf("disable update trigger: %v", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE audit_log DISABLE TRIGGER audit_log_no_delete`); err != nil {
		t.Fatalf("disable delete trigger: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `ALTER TABLE audit_log ENABLE TRIGGER audit_log_no_update`)
		_, _ = pool.Exec(ctx, `ALTER TABLE audit_log ENABLE TRIGGER audit_log_no_delete`)
	})

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

	if _, err := pool.Exec(ctx, `ALTER TABLE audit_log ENABLE TRIGGER audit_log_no_update`); err != nil {
		t.Fatalf("re-enable update trigger: %v", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE audit_log ENABLE TRIGGER audit_log_no_delete`); err != nil {
		t.Fatalf("re-enable delete trigger: %v", err)
	}

	if _, err := pool.Exec(ctx, `UPDATE audit_log SET action = 'tampered' WHERE id = $1`, id); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Errorf("superuser update error = %v, want append-only trigger rejection", err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM audit_log WHERE id = $1`, id); err == nil || !strings.Contains(err.Error(), "append-only") {
		t.Errorf("superuser delete error = %v, want append-only trigger rejection", err)
	}
}
