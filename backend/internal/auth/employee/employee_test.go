package employee

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type fakeRepository struct {
	writer    *fakeWriter
	committed bool
}
type fakeWriter struct {
	user       store.User
	createErr  error
	users      int
	roles      []string
	grantedBy  []uuid.UUID
	roleErr    error
	tokens     int
	audits     int
	auditActor *uuid.UUID
	auditAct   string
}

func (r *fakeRepository) WithinEmployeeCreationTransaction(_ context.Context, fn func(store.EmployeeCreationWriter) error) error {
	if err := fn(r.writer); err != nil {
		return err
	}
	r.committed = true
	return nil
}
func (w *fakeWriter) CreateUser(_ context.Context, _ store.CreateUserParams) (store.User, error) {
	w.users++
	if w.createErr != nil {
		return store.User{}, w.createErr
	}
	return w.user, nil
}
func (w *fakeWriter) AddUserRole(_ context.Context, _ uuid.UUID, role string, grantedBy uuid.UUID) error {
	w.roles = append(w.roles, role)
	w.grantedBy = append(w.grantedBy, grantedBy)
	return w.roleErr
}
func (w *fakeWriter) CreateInvitationToken(_ context.Context, _ store.CreateInvitationTokenParams) error {
	w.tokens++
	return nil
}
func (w *fakeWriter) InsertAuditEvent(_ context.Context, params store.InsertAuditEventParams) (store.AuditEvent, error) {
	w.audits++
	w.auditActor = params.ActorUserID
	w.auditAct = params.Action
	return store.AuditEvent{}, nil
}

type fakeHasher struct{ calls int }

func (h *fakeHasher) Hash(string) (string, error) { h.calls++; return "$argon2id$test", nil }

type fakePublisher struct {
	err    error
	events int
	event  any
}

func (p *fakePublisher) Publish(_ context.Context, _ string, event any) error {
	p.events++
	p.event = event
	return p.err
}

func testService(repo *fakeRepository, hasher *fakeHasher, publisher *fakePublisher) *Service {
	return New(repo, publisher, hasher, strings.NewReader(strings.Repeat("x", 64)), func() time.Time { return time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC) })
}

func TestRF001_AltaDevuelveCuentaPendienteConRolBaseYRolesSolicitados(t *testing.T) {
	actor := uuid.New()
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: uuid.New(), Email: "ana@example.com", DisplayName: "Ana", Status: "pending_verification"}}}
	hasher, publisher := &fakeHasher{}, &fakePublisher{}

	result, err := testService(repo, hasher, publisher).CreateEmployee(context.Background(), Input{
		Email: "ana@example.com", DisplayName: "Ana", Roles: []string{"contabilidad.analista"}, ActorUserID: actor,
	})
	if err != nil {
		t.Fatalf("CreateEmployee() error = %v", err)
	}
	if result.Status != "pending_verification" || result.Email != "ana@example.com" {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Roles) != 2 || !hasRole(result.Roles, "user") || !hasRole(result.Roles, "contabilidad.analista") {
		t.Fatalf("roles = %v, want [user contabilidad.analista]", result.Roles)
	}
	if !repo.committed || repo.writer.tokens != 1 || repo.writer.audits != 1 || publisher.events != 1 {
		t.Fatalf("side effects: committed=%t tokens=%d audits=%d events=%d", repo.committed, repo.writer.tokens, repo.writer.audits, publisher.events)
	}
	if len(repo.writer.roles) != 2 {
		t.Fatalf("AddUserRole calls = %v, want 2 role grants", repo.writer.roles)
	}
	for _, grantedBy := range repo.writer.grantedBy {
		if grantedBy != actor {
			t.Fatalf("role granted_by = %v, want actor %v", grantedBy, actor)
		}
	}
	if repo.writer.auditAct != "employee_created" || repo.writer.auditActor == nil || *repo.writer.auditActor != actor {
		t.Fatalf("audit action = %q actor = %v, want employee_created by %v", repo.writer.auditAct, repo.writer.auditActor, actor)
	}
	if hasher.calls != 1 {
		t.Fatalf("hasher calls = %d, want 1 (the admin never sets a password)", hasher.calls)
	}
}

func TestRF001_SoloRolUserNoLoDuplica(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: uuid.New(), Email: "ana@example.com", Status: "pending_verification"}}}
	result, err := testService(repo, &fakeHasher{}, &fakePublisher{}).CreateEmployee(context.Background(), Input{
		Email: "ana@example.com", DisplayName: "Ana", Roles: []string{"user"}, ActorUserID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("CreateEmployee() error = %v", err)
	}
	if len(result.Roles) != 1 || result.Roles[0] != "user" {
		t.Fatalf("roles = %v, want exactly [user]", result.Roles)
	}
	if len(repo.writer.roles) != 1 {
		t.Fatalf("AddUserRole calls = %v, want exactly one grant for user", repo.writer.roles)
	}
}

func TestRF001_RolDuplicadoNoDuplicaLaAsignacion(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: uuid.New(), Email: "ana@example.com", Status: "pending_verification"}}}
	result, err := testService(repo, &fakeHasher{}, &fakePublisher{}).CreateEmployee(context.Background(), Input{
		Email: "ana@example.com", DisplayName: "Ana", Roles: []string{"contabilidad.senior", "contabilidad.senior"}, ActorUserID: uuid.New(),
	})
	if err != nil {
		t.Fatalf("CreateEmployee() error = %v", err)
	}
	if len(result.Roles) != 2 {
		t.Fatalf("roles = %v, want exactly [user contabilidad.senior]", result.Roles)
	}
	if len(repo.writer.roles) != 2 {
		t.Fatalf("AddUserRole calls = %v, want exactly 2 grants despite the duplicate request", repo.writer.roles)
	}
}

func TestRF001_RolDesconocidoDevuelve400ConCampo(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{}}
	_, err := testService(repo, &fakeHasher{}, &fakePublisher{}).CreateEmployee(context.Background(), Input{
		Email: "ana@example.com", DisplayName: "Ana", Roles: []string{"operator"}, ActorUserID: uuid.New(),
	})
	var invalid *InvalidInputError
	if !errors.As(err, &invalid) || invalid.Field != "roles" {
		t.Fatalf("CreateEmployee() error = %v, want roles validation error", err)
	}
	if repo.writer.users != 0 {
		t.Fatalf("CreateUser was called with an invalid role request")
	}
}

func TestRF001_SinRolesDevuelve400(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{}}
	_, err := testService(repo, &fakeHasher{}, &fakePublisher{}).CreateEmployee(context.Background(), Input{
		Email: "ana@example.com", DisplayName: "Ana", Roles: nil, ActorUserID: uuid.New(),
	})
	var invalid *InvalidInputError
	if !errors.As(err, &invalid) || invalid.Field != "roles" {
		t.Fatalf("CreateEmployee() error = %v, want roles validation error", err)
	}
}

func TestRF001_CorreoDuplicadoDevuelve409SinRevelarExistencia(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{createErr: ErrEmailExists}}
	hasher := &fakeHasher{}
	_, err := testService(repo, hasher, &fakePublisher{}).CreateEmployee(context.Background(), Input{
		Email: "ana@example.com", DisplayName: "Ana", Roles: []string{"user"}, ActorUserID: uuid.New(),
	})
	if !errors.Is(err, ErrEmailExists) {
		t.Fatalf("CreateEmployee() error = %v, want ErrEmailExists", err)
	}
	if hasher.calls != 1 {
		t.Fatalf("hasher calls = %d, want 1", hasher.calls)
	}
}

func TestRF001_FalloDelBrokerRevierteYDevuelve503(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: uuid.New(), Email: "ana@example.com", Status: "pending_verification"}}}
	_, err := testService(repo, &fakeHasher{}, &fakePublisher{err: errors.New("broker unavailable")}).CreateEmployee(context.Background(), Input{
		Email: "ana@example.com", DisplayName: "Ana", Roles: []string{"user"}, ActorUserID: uuid.New(),
	})
	if !errors.Is(err, ErrPublish) {
		t.Fatalf("CreateEmployee() error = %v, want ErrPublish", err)
	}
	if repo.committed {
		t.Fatal("transaction committed after publisher failure")
	}
}

// RF-012: the invitation event must carry the raw invitation token (in
// cleartext) so the worker can render the acceptance link; only the database
// ever sees its SHA-256 hash.
func TestRF012_LaInvitacionSeEncolaConElTokenEnClaro(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: uuid.New(), Email: "ana@example.com", DisplayName: "Ana", Status: "pending_verification"}}}
	publisher := &fakePublisher{}
	if _, err := testService(repo, &fakeHasher{}, publisher).CreateEmployee(context.Background(), Input{
		Email: "ana@example.com", DisplayName: "Ana", Roles: []string{"user"}, ActorUserID: uuid.New(),
	}); err != nil {
		t.Fatalf("CreateEmployee() error = %v", err)
	}
	event, ok := publisher.event.(events.UserInvited)
	if !ok {
		t.Fatalf("published event type = %T, want events.UserInvited", publisher.event)
	}
	if event.Data.InvitationToken == "" {
		t.Fatal("event carries no invitation token")
	}
	if event.EventType != events.TypeUserInvited {
		t.Fatalf("EventType = %q, want %q", event.EventType, events.TypeUserInvited)
	}
}
