//go:build integration

package audit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
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
	mustExec(t, pool, "disable update trigger", `ALTER TABLE audit_log DISABLE TRIGGER audit_log_no_update`)
	mustExec(t, pool, "disable delete trigger", `ALTER TABLE audit_log DISABLE TRIGGER audit_log_no_delete`)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `ALTER TABLE audit_log ENABLE TRIGGER audit_log_no_update`)
		_, _ = pool.Exec(ctx, `ALTER TABLE audit_log ENABLE TRIGGER audit_log_no_delete`)
	})

	mustExec(t, pool, "set application role", `SET ROLE identity_app`)
	_, updateErr := pool.Exec(ctx, `UPDATE audit_log SET action = 'tampered' WHERE id = $1`, id)
	_, deleteErr := pool.Exec(ctx, `DELETE FROM audit_log WHERE id = $1`, id)
	mustExec(t, pool, "reset application role", `RESET ROLE`)
	for operation, err := range map[string]error{"UPDATE": updateErr, "DELETE": deleteErr} {
		requireErrorContaining(t, operation+" error", err, "permission denied")
	}

	mustExec(t, pool, "re-enable update trigger", `ALTER TABLE audit_log ENABLE TRIGGER audit_log_no_update`)
	mustExec(t, pool, "re-enable delete trigger", `ALTER TABLE audit_log ENABLE TRIGGER audit_log_no_delete`)

	_, err := pool.Exec(ctx, `UPDATE audit_log SET action = 'tampered' WHERE id = $1`, id)
	requireErrorContaining(t, "superuser update error", err, "append-only")
	_, err = pool.Exec(ctx, `DELETE FROM audit_log WHERE id = $1`, id)
	requireErrorContaining(t, "superuser delete error", err, "append-only")
}

// mustExec runs a statement that has to succeed and fails the test with label otherwise.
func mustExec(t *testing.T, pool *pgxpool.Pool, label, statement string) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), statement); err != nil {
		t.Fatalf("%s: %v", label, err)
	}
}

// requireErrorContaining reports (without stopping the test) unless err is non-nil and mentions want.
func requireErrorContaining(t *testing.T, label string, err error, want string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("%s = %v, want %s", label, err, want)
	}
}
