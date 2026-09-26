//go:build integration

package store_test

import (
	"context"
	"net/netip"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

func TestRF017_CuentaFallosDeLoginPorIPDentroDeLaVentana(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}

	ip := netip.MustParseAddr("198.51.100.7")
	otherIP := netip.MustParseAddr("203.0.113.55")

	insertFailure := func(addr *netip.Addr, action string) {
		if _, err := repository.InsertAuditEvent(ctx, store.InsertAuditEventParams{
			Action:   action,
			IP:       addr,
			Metadata: []byte(`{}`),
		}); err != nil {
			t.Fatalf("InsertAuditEvent(%s): %v", action, err)
		}
	}

	insertFailure(&ip, "login_failed")
	insertFailure(&ip, "login_failed")
	insertFailure(&otherIP, "login_failed") // different IP, must not count
	insertFailure(&ip, "login_succeeded")   // different action, must not count

	// audit_log is append-only (RF-011, invariante 5): a BEFORE UPDATE trigger
	// rejects UPDATE for every role, including the owner used here, so a stale
	// failure outside the lookback window is seeded with an explicit past
	// created_at at INSERT time instead of backdating an existing row.
	if _, err := pool.Exec(ctx, `INSERT INTO audit_log (action, ip, metadata, created_at) VALUES ('login_failed', $1, '{}'::jsonb, now() - interval '1 hour')`, &ip); err != nil {
		t.Fatalf("seed stale failure: %v", err)
	}

	since := time.Now().Add(-15 * time.Minute)
	var countedFailures int64
	err = repository.WithinLoginTransaction(ctx, func(writer login.Writer) error {
		var innerErr error
		countedFailures, innerErr = writer.CountLoginFailuresByIP(ctx, ip, since)
		return innerErr
	})
	if err != nil {
		t.Fatalf("WithinLoginTransaction/CountLoginFailuresByIP: %v", err)
	}
	if countedFailures != 2 {
		t.Fatalf("CountLoginFailuresByIP = %d, want 2 (excluding the other IP, the success, and the stale failure)", countedFailures)
	}
}

func TestRF017_DesbloqueaUsuarioReactivaLaCuentaBloqueada(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	ctx := context.Background()
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}

	created, err := repository.CreateUser(ctx, store.CreateUserParams{
		Email:        "lockout-writer@example.test",
		PasswordHash: "$argon2id$fixed-test-value",
		DisplayName:  "Lockout Writer",
	})
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE id = $1`, created.ID); err != nil {
		t.Fatalf("activate user: %v", err)
	}

	lockedUntil := time.Now().Add(15 * time.Minute)
	err = repository.WithinLoginTransaction(ctx, func(writer login.Writer) error {
		return writer.LockLoginUser(ctx, created.ID, lockedUntil)
	})
	if err != nil {
		t.Fatalf("WithinLoginTransaction/LockLoginUser: %v", err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM users WHERE id = $1`, created.ID).Scan(&status); err != nil {
		t.Fatalf("read locked status: %v", err)
	}
	if status != "locked" {
		t.Fatalf("status after LockLoginUser = %q, want locked", status)
	}

	err = repository.WithinLoginTransaction(ctx, func(writer login.Writer) error {
		return writer.UnlockLoginUser(ctx, created.ID)
	})
	if err != nil {
		t.Fatalf("WithinLoginTransaction/UnlockLoginUser: %v", err)
	}
	var lockedUntilAfter *time.Time
	if err := pool.QueryRow(ctx, `SELECT status, locked_until FROM users WHERE id = $1`, created.ID).Scan(&status, &lockedUntilAfter); err != nil {
		t.Fatalf("read unlocked status: %v", err)
	}
	if status != "active" || lockedUntilAfter != nil {
		t.Fatalf("after UnlockLoginUser status=%q locked_until=%v, want active/nil", status, lockedUntilAfter)
	}

	// UnlockLoginUser only matches WHERE status = 'locked'; calling it again on an
	// already-active account must stay a harmless no-op, not an error.
	err = repository.WithinLoginTransaction(ctx, func(writer login.Writer) error {
		return writer.UnlockLoginUser(ctx, created.ID)
	})
	if err != nil {
		t.Fatalf("WithinLoginTransaction/UnlockLoginUser (idempotent call): %v", err)
	}
}
