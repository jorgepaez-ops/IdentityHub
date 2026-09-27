package invitation

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type fakeRepository struct {
	writer    *fakeWriter
	committed bool
}
type fakeWriter struct {
	user        store.User
	consumeErr  error
	consumed    int
	consumedArg store.ConsumeInvitationTokenParams
	audits      int
	auditActor  *uuid.UUID
	auditAct    string
}

func (r *fakeRepository) WithinInvitationAcceptanceTransaction(_ context.Context, fn func(store.InvitationAcceptanceWriter) error) error {
	if err := fn(r.writer); err != nil {
		return err
	}
	r.committed = true
	return nil
}
func (w *fakeWriter) ConsumeInvitationToken(_ context.Context, params store.ConsumeInvitationTokenParams) (store.User, error) {
	w.consumed++
	w.consumedArg = params
	if w.consumeErr != nil {
		return store.User{}, w.consumeErr
	}
	return w.user, nil
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

func TestRF002_AceptarActivaLaCuentaYFijaLaContrasena(t *testing.T) {
	actorID := uuid.New()
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: actorID, Email: "ada@example.com", DisplayName: "Ada", Status: "active"}}}
	hasher, publisher := &fakeHasher{}, &fakePublisher{}

	err := New(repo, publisher, hasher).Accept(context.Background(), Input{Token: "invitation-token", Password: "correct horse battery"})
	if err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	if !repo.committed || repo.writer.consumed != 1 || repo.writer.audits != 1 || publisher.events != 1 {
		t.Fatalf("side effects: committed=%t consumed=%d audits=%d events=%d", repo.committed, repo.writer.consumed, repo.writer.audits, publisher.events)
	}
	if hasher.calls != 1 {
		t.Fatalf("hasher calls = %d, want 1", hasher.calls)
	}
	if repo.writer.consumedArg.PasswordHash != "$argon2id$test" {
		t.Fatalf("PasswordHash passed to ConsumeInvitationToken = %q, want the Argon2id hash", repo.writer.consumedArg.PasswordHash)
	}
	if repo.writer.auditAct != "invitation_accepted" || repo.writer.auditActor == nil || *repo.writer.auditActor != actorID {
		t.Fatalf("audit action = %q actor = %v, want invitation_accepted by %v", repo.writer.auditAct, repo.writer.auditActor, actorID)
	}
}

func TestRF002_ContrasenaCortaDevuelve400ConCampo(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{}}
	err := New(repo, &fakePublisher{}, &fakeHasher{}).Accept(context.Background(), Input{Token: "invitation-token", Password: "short"})
	var invalid *InvalidInputError
	if !errors.As(err, &invalid) || invalid.Field != "password" {
		t.Fatalf("Accept() error = %v, want password validation error", err)
	}
	if repo.writer.consumed != 0 {
		t.Fatal("token was consumed despite an invalid password")
	}
}

func TestRF002_TokenVacioDevuelve400(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{}}
	err := New(repo, &fakePublisher{}, &fakeHasher{}).Accept(context.Background(), Input{Token: "", Password: "correct horse battery"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Accept() error = %v, want ErrInvalidInput", err)
	}
}

func TestRF002_LaInvitacionEsDeUnSoloUso(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{consumeErr: ErrTokenInvalid}}
	err := New(repo, &fakePublisher{}, &fakeHasher{}).Accept(context.Background(), Input{Token: "used-token", Password: "correct horse battery"})
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Accept() error = %v, want ErrTokenInvalid", err)
	}
	if repo.committed {
		t.Fatal("transaction committed after consuming an already-used token")
	}
}

func TestRF002_TokenExpiradoDevuelve410(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{consumeErr: ErrTokenInvalid}}
	err := New(repo, &fakePublisher{}, &fakeHasher{}).Accept(context.Background(), Input{Token: "expired-token", Password: "correct horse battery"})
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Accept() error = %v, want ErrTokenInvalid", err)
	}
}

func TestRF002_ErrorDelBrokerDevuelve503(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: uuid.New(), Email: "ada@example.com", DisplayName: "Ada", Status: "active"}}}
	err := New(repo, &fakePublisher{err: errors.New("broker unavailable")}, &fakeHasher{}).Accept(context.Background(), Input{Token: "invitation-token", Password: "correct horse battery"})
	if !errors.Is(err, ErrPublish) {
		t.Fatalf("Accept() error = %v, want ErrPublish", err)
	}
	if repo.committed {
		t.Fatal("transaction committed after publisher failure")
	}
}

func TestRNF012_AceptarNoIncluyeElTokenEnElEvento(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: uuid.New(), Email: "ada@example.com", DisplayName: "Ada", Status: "active"}}}
	publisher := &fakePublisher{}
	token := base64.RawURLEncoding.EncodeToString([]byte("secret-invitation-token"))
	if err := New(repo, publisher, &fakeHasher{}).Accept(context.Background(), Input{Token: token, Password: "correct horse battery"}); err != nil {
		t.Fatalf("Accept() error = %v", err)
	}
	encoded, err := json.Marshal(publisher.event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if strings.Contains(string(encoded), token) {
		t.Fatalf("invitation accepted event exposes token: %s", encoded)
	}
	event, ok := publisher.event.(events.EmailVerified)
	if !ok {
		t.Fatalf("published event type = %T, want events.EmailVerified (account activation reuses the same notification)", publisher.event)
	}
	if event.EventType != events.TypeEmailVerified {
		t.Fatalf("EventType = %q, want %q", event.EventType, events.TypeEmailVerified)
	}
}
