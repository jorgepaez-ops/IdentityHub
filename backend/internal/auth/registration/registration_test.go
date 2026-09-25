package registration

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type fakeRepository struct {
	writer    *fakeWriter
	committed bool
}
type fakeWriter struct {
	user      store.User
	createErr error
	users     int
	tokens    int
	audits    int
}

func (r *fakeRepository) WithinRegistrationTransaction(_ context.Context, fn func(store.RegistrationWriter) error) error {
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
func (w *fakeWriter) CreateVerificationToken(_ context.Context, _ store.CreateVerificationTokenParams) error {
	w.tokens++
	return nil
}
func (w *fakeWriter) InsertAuditEvent(_ context.Context, _ store.InsertAuditEventParams) (store.AuditEvent, error) {
	w.audits++
	return store.AuditEvent{}, nil
}

type fakeHasher struct{ calls int }

func (h *fakeHasher) Hash(string) (string, error) { h.calls++; return "$argon2id$test", nil }

type fakePublisher struct {
	err    error
	events int
}

func (p *fakePublisher) Publish(_ context.Context, _ string, _ any) error { p.events++; return p.err }

func testService(repo *fakeRepository, hasher *fakeHasher, publisher *fakePublisher) *Service {
	return New(repo, publisher, hasher, strings.NewReader(strings.Repeat("x", 32)), func() time.Time { return time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC) })
}

func TestRF001_RegistroDevuelve201YCuentaPendiente(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: uuid.New(), Email: "ada@example.com", Status: "pending_verification"}}}
	hasher, publisher := &fakeHasher{}, &fakePublisher{}
	result, err := testService(repo, hasher, publisher).Register(context.Background(), Input{Email: "ada@example.com", Password: "correct horse battery", DisplayName: "Ada"})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if result.Status != "pending_verification" || result.Email != "ada@example.com" {
		t.Fatalf("result = %+v", result)
	}
	if !repo.committed || repo.writer.tokens != 1 || repo.writer.audits != 1 || publisher.events != 1 {
		t.Fatalf("side effects: committed=%t tokens=%d audits=%d events=%d", repo.committed, repo.writer.tokens, repo.writer.audits, publisher.events)
	}
	if hasher.calls != 1 {
		t.Fatalf("hasher calls = %d, want 1", hasher.calls)
	}
}

func TestRF001_ContrasenaCortaDevuelve400ConCampo(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{}}
	_, err := testService(repo, &fakeHasher{}, &fakePublisher{}).Register(context.Background(), Input{Email: "ada@example.com", Password: "short", DisplayName: "Ada"})
	var invalid *InvalidInputError
	if !errors.As(err, &invalid) || invalid.Field != "password" {
		t.Fatalf("Register() error = %v, want password validation error", err)
	}
}

func TestRF001_CorreoDuplicadoDevuelve409SinRevelarExistencia(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{createErr: ErrEmailExists}}
	hasher := &fakeHasher{}
	_, err := testService(repo, hasher, &fakePublisher{}).Register(context.Background(), Input{Email: "ada@example.com", Password: "correct horse battery", DisplayName: "Ada"})
	if !errors.Is(err, ErrEmailExists) {
		t.Fatalf("Register() error = %v, want ErrEmailExists", err)
	}
	if hasher.calls != 1 {
		t.Fatalf("hasher calls = %d, want 1", hasher.calls)
	}
}

func TestRF001_FalloDelBrokerRevierteYDevuelve503(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: uuid.New(), Email: "ada@example.com", Status: "pending_verification"}}}
	_, err := testService(repo, &fakeHasher{}, &fakePublisher{err: errors.New("broker unavailable")}).Register(context.Background(), Input{Email: "ada@example.com", Password: "correct horse battery", DisplayName: "Ada"})
	if !errors.Is(err, ErrPublish) {
		t.Fatalf("Register() error = %v, want ErrPublish", err)
	}
	if repo.committed {
		t.Fatal("transaction committed after publisher failure")
	}
}
