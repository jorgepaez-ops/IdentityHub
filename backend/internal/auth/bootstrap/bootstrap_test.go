package bootstrap

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type fakeRepository struct {
	writer    *fakeWriter
	committed bool
}

type fakeWriter struct {
	adminExists bool
	user        store.User
	userErr     error
	locked      int
	users       int
	roles       []string
	tokens      int
	userRoles   []string
	liveInvite  bool
	invalidated []uuid.UUID
	audits      []store.InsertAuditEventParams
}

func (r *fakeRepository) WithinBootstrapAdminTransaction(_ context.Context, fn func(store.BootstrapAdminWriter) error) error {
	if err := fn(r.writer); err != nil {
		return err
	}
	r.committed = true
	return nil
}

func (w *fakeWriter) LockBootstrapAdmin(context.Context) error        { w.locked++; return nil }
func (w *fakeWriter) ActiveAdminExists(context.Context) (bool, error) { return w.adminExists, nil }
func (w *fakeWriter) GetUserByEmail(context.Context, string) (store.User, error) {
	return w.user, w.userErr
}
func (w *fakeWriter) CreateUser(_ context.Context, params store.CreateUserParams) (store.User, error) {
	w.users++
	if w.user.ID == uuid.Nil {
		w.user = store.User{ID: uuid.New(), Email: params.Email, DisplayName: params.DisplayName, Status: "pending_verification", CreatedAt: time.Now()}
	}
	return w.user, nil
}
func (w *fakeWriter) AddBootstrapUserRole(_ context.Context, _ uuid.UUID, role string) error {
	w.roles = append(w.roles, role)
	return nil
}
func (w *fakeWriter) ListRolesForUser(context.Context, uuid.UUID) ([]string, error) {
	return w.userRoles, nil
}
func (w *fakeWriter) HasLiveInvitationToken(context.Context, uuid.UUID) (bool, error) {
	return w.liveInvite, nil
}
func (w *fakeWriter) InvalidateInvitationTokens(_ context.Context, id uuid.UUID) error {
	w.invalidated = append(w.invalidated, id)
	return nil
}
func (w *fakeWriter) CreateInvitationToken(context.Context, store.CreateInvitationTokenParams) error {
	w.tokens++
	return nil
}
func (w *fakeWriter) InsertAuditEvent(_ context.Context, params store.InsertAuditEventParams) (store.AuditEvent, error) {
	w.audits = append(w.audits, params)
	return store.AuditEvent{}, nil
}

type fakeHasher struct{ calls int }

func (h *fakeHasher) Hash(string) (string, error) { h.calls++; return "$argon2id$test", nil }

type fakePublisher struct {
	err    error
	events []any
}

func (p *fakePublisher) Publish(_ context.Context, _ string, event any) error {
	p.events = append(p.events, event)
	return p.err
}

func newTestService(repo *fakeRepository, publisher *fakePublisher) *Service {
	return New(repo, publisher, &fakeHasher{}, strings.NewReader(strings.Repeat("x", 64)), func() time.Time {
		return time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	})
}

func TestBootstrap_CreaPrimerAdminPendienteConInvitacionYAuditoriaDeSistema(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{userErr: pgx.ErrNoRows}}
	publisher := &fakePublisher{}
	outcome, err := newTestService(repo, publisher).Ensure(context.Background(), "first-admin@example.test")
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if outcome != OutcomeCreated {
		t.Fatalf("outcome = %q, want %q", outcome, OutcomeCreated)
	}
	if !repo.committed || repo.writer.locked != 1 || repo.writer.users != 1 || repo.writer.tokens != 1 || len(publisher.events) != 1 {
		t.Fatalf("side effects: committed=%t locks=%d users=%d tokens=%d events=%d", repo.committed, repo.writer.locked, repo.writer.users, repo.writer.tokens, len(publisher.events))
	}
	if got := repo.writer.roles; len(got) != 2 || got[0] != "user" || got[1] != "admin" {
		t.Fatalf("roles = %v, want [user admin]", got)
	}
	if len(repo.writer.audits) != 1 || repo.writer.audits[0].ActorUserID != nil || string(repo.writer.audits[0].Metadata) != `{"actor":"system/bootstrap"}` {
		t.Fatalf("audit = %#v, want system/bootstrap actor metadata", repo.writer.audits)
	}
	event, ok := publisher.events[0].(events.UserInvited)
	if !ok || event.EventType != events.TypeUserInvited || event.Data.InvitationToken == "" {
		t.Fatalf("event = %#v, want user.invited with token", publisher.events[0])
	}
}

func TestBootstrap_NoOpParaAdminExistenteOCorreoYaOcupado(t *testing.T) {
	for _, tt := range []struct {
		name   string
		writer *fakeWriter
		want   Outcome
	}{
		{name: "existing admin", writer: &fakeWriter{adminExists: true}, want: OutcomeAdminExists},
		{name: "email owned by non admin", writer: &fakeWriter{user: store.User{ID: uuid.New(), Status: "pending_verification"}, userRoles: []string{"user"}}, want: OutcomeEmailConflict},
		{name: "email owned by active non admin", writer: &fakeWriter{user: store.User{ID: uuid.New(), Status: "active"}, userRoles: []string{"user"}}, want: OutcomeEmailConflict},
		{name: "email owned by locked admin-role account", writer: &fakeWriter{user: store.User{ID: uuid.New(), Status: "locked"}, userRoles: []string{"user", "admin"}}, want: OutcomeEmailConflict},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := &fakeRepository{writer: tt.writer}
			publisher := &fakePublisher{}
			outcome, err := newTestService(repo, publisher).Ensure(context.Background(), "first-admin@example.test")
			if err != nil {
				t.Fatalf("Ensure() error = %v", err)
			}
			if outcome != tt.want {
				t.Fatalf("outcome = %q, want %q", outcome, tt.want)
			}
			if !repo.committed || repo.writer.users != 0 || repo.writer.tokens != 0 || len(publisher.events) != 0 {
				t.Fatalf("unexpected writes or events: %+v", repo.writer)
			}
		})
	}
}

func TestBootstrap_ElErrorDelBrokerRevierteLaCreacion(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{userErr: pgx.ErrNoRows}}
	_, err := newTestService(repo, &fakePublisher{err: errors.New("broker unavailable")}).Ensure(context.Background(), "first-admin@example.test")
	if !errors.Is(err, ErrPublish) {
		t.Fatalf("Ensure() error = %v, want ErrPublish", err)
	}
	if repo.committed {
		t.Fatal("transaction committed after publish failure")
	}
}

func TestBootstrap_ErrorDeBaseNoFiltraCorreoYNuncaContinua(t *testing.T) {
	const email = "first-admin@example.test"
	repo := &fakeRepository{writer: &fakeWriter{userErr: errors.New("database failure for " + email)}}
	outcome, err := newTestService(repo, &fakePublisher{}).Ensure(context.Background(), email)
	if err == nil {
		t.Fatal("Ensure() returned nil on a database error")
	}
	if outcome != "" {
		t.Fatalf("outcome = %q, want empty on failure", outcome)
	}
	if strings.Contains(err.Error(), email) {
		t.Fatalf("bootstrap error leaked configured email: %v", err)
	}
	if repo.committed {
		t.Fatal("transaction committed after database error")
	}
}

func TestBootstrap_AdminPendienteRecibeInvitacionDeReemplazoSinDuplicarCuenta(t *testing.T) {
	const email = "first-admin@example.test"
	pending := store.User{ID: uuid.New(), Email: email, DisplayName: "Bootstrap Admin", Status: "pending_verification"}
	repo := &fakeRepository{writer: &fakeWriter{user: pending, userRoles: []string{"admin", "user"}}}
	publisher := &fakePublisher{}
	outcome, err := newTestService(repo, publisher).Ensure(context.Background(), email)
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if outcome != OutcomeInvitationReissued {
		t.Fatalf("outcome = %q, want %q", outcome, OutcomeInvitationReissued)
	}
	w := repo.writer
	if !repo.committed || w.users != 0 || len(w.roles) != 0 || w.tokens != 1 || len(publisher.events) != 1 {
		t.Fatalf("side effects: committed=%t users=%d roles=%v tokens=%d events=%d", repo.committed, w.users, w.roles, w.tokens, len(publisher.events))
	}
	if len(w.invalidated) != 1 || w.invalidated[0] != pending.ID {
		t.Fatalf("invalidated = %v, want [%s]", w.invalidated, pending.ID)
	}
	if len(w.audits) != 1 || w.audits[0].ActorUserID != nil || w.audits[0].Action != "bootstrap_admin_invitation_reissued" || string(w.audits[0].Metadata) != `{"actor":"system/bootstrap"}` {
		t.Fatalf("audit = %#v, want system/bootstrap reissue", w.audits)
	}
	event, ok := publisher.events[0].(events.UserInvited)
	if !ok || event.Data.UserID != pending.ID || event.Data.Email != email || event.Data.InvitationToken == "" {
		t.Fatalf("event = %#v, want user.invited for the pending admin", publisher.events[0])
	}
}

func TestBootstrap_ElErrorDelBrokerRevierteLaInvitacionDeReemplazo(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: uuid.New(), Status: "pending_verification"}, userRoles: []string{"admin"}}}
	_, err := newTestService(repo, &fakePublisher{err: errors.New("broker unavailable")}).Ensure(context.Background(), "first-admin@example.test")
	if !errors.Is(err, ErrPublish) {
		t.Fatalf("Ensure() error = %v, want ErrPublish", err)
	}
	if repo.committed {
		t.Fatal("transaction committed after publish failure")
	}
}

func TestBootstrap_AdminPendienteConInvitacionVigenteNoSeReinvita(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: uuid.New(), Status: "pending_verification"}, userRoles: []string{"admin"}, liveInvite: true}}
	publisher := &fakePublisher{}
	outcome, err := newTestService(repo, publisher).Ensure(context.Background(), "first-admin@example.test")
	if err != nil {
		t.Fatalf("Ensure() error = %v", err)
	}
	if outcome != OutcomeInvitationPending {
		t.Fatalf("outcome = %q, want %q", outcome, OutcomeInvitationPending)
	}
	if w := repo.writer; w.tokens != 0 || len(w.invalidated) != 0 || len(w.audits) != 0 || len(publisher.events) != 0 {
		t.Fatalf("unexpected writes: %+v events=%d", w, len(publisher.events))
	}
}
