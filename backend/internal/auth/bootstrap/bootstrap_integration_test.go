//go:build integration

package bootstrap

import (
	"context"
	"crypto/rand"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/store"
	"github.com/jorgepaez/identity-hub/internal/testdb"
)

type integrationHasher struct{}

func (integrationHasher) Hash(value string) (string, error) { return password.Hash(value) }

type integrationPublisher struct {
	mu     sync.Mutex
	events int
	err    error
}

func (p *integrationPublisher) Publish(context.Context, string, any) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events++
	return p.err
}

func (p *integrationPublisher) setErr(err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.err = err
}

func TestBootstrap_IniciosConcurrentesCreanUnaCuentaYUnaInvitacion(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	publisher := &integrationPublisher{}
	service := New(repository, publisher, integrationHasher{}, rand.Reader, time.Now)

	const concurrentStarts = 8
	start := make(chan struct{})
	errs := make(chan error, concurrentStarts)
	var wg sync.WaitGroup
	for range concurrentStarts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := service.Ensure(context.Background(), "first-admin@example.test")
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Ensure: %v", err)
		}
	}

	ctx := context.Background()
	var users, invitations int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE email = $1`, "first-admin@example.test").Scan(&users); err != nil {
		t.Fatalf("count bootstrap users: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM verification_tokens v JOIN users u ON u.id = v.user_id WHERE u.email = $1 AND v.purpose = 'invitation'`, "first-admin@example.test").Scan(&invitations); err != nil {
		t.Fatalf("count bootstrap invitations: %v", err)
	}
	if users != 1 || invitations != 1 {
		t.Fatalf("bootstrap rows = users:%d invitations:%d, want 1 each", users, invitations)
	}
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	if publisher.events != 1 {
		t.Fatalf("published invitations = %d, want 1", publisher.events)
	}
}

func TestBootstrap_AdminPendienteSeReinvitaYElTokenViejoQuedaInvalido(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	publisher := &integrationPublisher{}
	service := New(repository, publisher, integrationHasher{}, rand.Reader, time.Now)
	ctx := context.Background()
	const email = "first-admin@example.test"

	if outcome, err := service.Ensure(ctx, email); err != nil || outcome != OutcomeCreated {
		t.Fatalf("first Ensure = %q, %v; want created", outcome, err)
	}
	// The 24 h invitation lapses without being accepted.
	if _, err := pool.Exec(ctx, `UPDATE verification_tokens SET expires_at = now() - interval '1 hour' WHERE purpose = 'invitation'`); err != nil {
		t.Fatalf("expire invitation: %v", err)
	}
	outcome, err := service.Ensure(ctx, email)
	if err != nil || outcome != OutcomeInvitationReissued {
		t.Fatalf("second Ensure = %q, %v; want invitation_reissued", outcome, err)
	}
	var users, unused, total int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users WHERE email = $1`, email).Scan(&users); err != nil {
		t.Fatalf("count users: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE used_at IS NULL), count(*) FROM verification_tokens WHERE purpose = 'invitation'`).Scan(&unused, &total); err != nil {
		t.Fatalf("count tokens: %v", err)
	}
	if users != 1 || total != 2 || unused != 1 {
		t.Fatalf("rows = users:%d tokens:%d unused:%d, want 1/2/1", users, total, unused)
	}
	var audits int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action = 'bootstrap_admin_invitation_reissued' AND actor_user_id IS NULL`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("reissue audit entries = %d, err %v; want 1", audits, err)
	}
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	if publisher.events != 2 {
		t.Fatalf("published invitations = %d, want 2", publisher.events)
	}
}

func TestBootstrap_AdminActivoHaceNoOp(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	publisher := &integrationPublisher{}
	service := New(repository, publisher, integrationHasher{}, rand.Reader, time.Now)
	ctx := context.Background()
	const email = "first-admin@example.test"
	if _, err := service.Ensure(ctx, email); err != nil {
		t.Fatalf("Ensure: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET status = 'active' WHERE email = $1`, email); err != nil {
		t.Fatalf("activate admin: %v", err)
	}
	outcome, err := service.Ensure(ctx, email)
	if err != nil || outcome != OutcomeAdminExists {
		t.Fatalf("Ensure = %q, %v; want admin_exists", outcome, err)
	}
	publisher.mu.Lock()
	defer publisher.mu.Unlock()
	if publisher.events != 1 {
		t.Fatalf("published invitations = %d, want 1", publisher.events)
	}
}

func TestBootstrap_AdminBloqueadoODeshabilitadoHaceNoOp(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	for _, status := range []string{"locked", "disabled"} {
		t.Run(status, func(t *testing.T) {
			pool := testdb.New(t)
			repository, err := store.NewWithPool(pool)
			if err != nil {
				t.Fatalf("NewWithPool: %v", err)
			}
			publisher := &integrationPublisher{}
			service := New(repository, publisher, integrationHasher{}, rand.Reader, time.Now)
			ctx := context.Background()
			const email = "first-admin@example.test"
			if _, err := service.Ensure(ctx, email); err != nil {
				t.Fatalf("Ensure: %v", err)
			}
			if _, err := pool.Exec(ctx, `UPDATE users SET status = $2 WHERE email = $1`, email, status); err != nil {
				t.Fatalf("set status: %v", err)
			}
			// A different configured email must not create a second admin.
			outcome, err := service.Ensure(ctx, "another-admin@example.test")
			if err != nil || outcome != OutcomeAdminExists {
				t.Fatalf("Ensure = %q, %v; want admin_exists", outcome, err)
			}
			var users int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&users); err != nil || users != 1 {
				t.Fatalf("users = %d, err %v; want 1", users, err)
			}
			publisher.mu.Lock()
			defer publisher.mu.Unlock()
			if publisher.events != 1 {
				t.Fatalf("published invitations = %d, want 1", publisher.events)
			}
		})
	}
}

func TestBootstrap_FalloDePublicacionTrasCommitDejaCuentaPendienteSinInvitacionVigenteYElSiguienteArranqueReemite(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires a PostgreSQL server")
	}
	pool := testdb.New(t)
	repository, err := store.NewWithPool(pool)
	if err != nil {
		t.Fatalf("NewWithPool: %v", err)
	}
	publisher := &integrationPublisher{}
	publisher.setErr(errors.New("broker unavailable"))
	service := New(repository, publisher, integrationHasher{}, rand.Reader, time.Now)
	ctx := context.Background()
	const email = "first-admin@example.test"

	outcome, err := service.Ensure(ctx, email)
	if err != nil || outcome != OutcomeInvitationUndelivered {
		t.Fatalf("first Ensure = %q, %v; want invitation_undelivered", outcome, err)
	}
	var status string
	var live int
	if err := pool.QueryRow(ctx, `SELECT status FROM users WHERE email = $1`, email).Scan(&status); err != nil || status != "pending_verification" {
		t.Fatalf("status = %q, err %v; want pending_verification", status, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM verification_tokens WHERE purpose = 'invitation' AND used_at IS NULL AND expires_at > now()`).Scan(&live); err != nil || live != 0 {
		t.Fatalf("live invitations = %d, err %v; want 0", live, err)
	}

	publisher.setErr(nil)
	outcome, err = service.Ensure(ctx, email)
	if err != nil || outcome != OutcomeInvitationReissued {
		t.Fatalf("second Ensure = %q, %v; want invitation_reissued", outcome, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM verification_tokens WHERE purpose = 'invitation' AND used_at IS NULL AND expires_at > now()`).Scan(&live); err != nil || live != 1 {
		t.Fatalf("live invitations after reissue = %d, err %v; want 1", live, err)
	}
}
