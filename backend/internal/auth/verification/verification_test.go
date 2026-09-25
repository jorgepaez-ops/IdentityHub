package verification

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type fakeRepository struct {
	writer    *fakeWriter
	committed bool
}

func (r *fakeRepository) WithinEmailVerificationTransaction(_ context.Context, fn func(store.EmailVerificationWriter) error) error {
	if err := fn(r.writer); err != nil {
		return err
	}
	r.committed = true
	return nil
}

type fakeWriter struct {
	user       store.User
	consumeErr error
	consumed   int
	audits     int
}

func (w *fakeWriter) ConsumeEmailVerificationToken(_ context.Context, _ []byte) (store.User, error) {
	w.consumed++
	if w.consumeErr != nil {
		return store.User{}, w.consumeErr
	}
	return w.user, nil
}

func (w *fakeWriter) InsertAuditEvent(_ context.Context, _ store.InsertAuditEventParams) (store.AuditEvent, error) {
	w.audits++
	return store.AuditEvent{}, nil
}

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

func TestRF002_VerificarActivaLaCuenta(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: uuid.New(), Email: "ada@example.com", DisplayName: "Ada", Status: "active"}}}
	publisher := &fakePublisher{}

	err := New(repo, publisher).Verify(context.Background(), Input{Token: "verification-token"})
	if err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	if !repo.committed || repo.writer.consumed != 1 || repo.writer.audits != 1 || publisher.events != 1 {
		t.Fatalf("side effects: committed=%t consumed=%d audits=%d events=%d", repo.committed, repo.writer.consumed, repo.writer.audits, publisher.events)
	}
}

func TestRF002_EnlaceDeUnSoloUso(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{consumeErr: ErrTokenInvalid}}
	err := New(repo, &fakePublisher{}).Verify(context.Background(), Input{Token: "used-token"})
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Verify() error = %v, want ErrTokenInvalid", err)
	}
	if repo.committed {
		t.Fatal("transaction committed after consuming an already-used token")
	}
}

func TestRF002_TokenExpiradoDevuelve410(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{consumeErr: ErrTokenInvalid}}
	err := New(repo, &fakePublisher{}).Verify(context.Background(), Input{Token: "expired-token"})
	if !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("Verify() error = %v, want ErrTokenInvalid", err)
	}
}

func TestRNF012_VerificacionNoIncluyeTokenEnElEvento(t *testing.T) {
	repo := &fakeRepository{writer: &fakeWriter{user: store.User{ID: uuid.New(), Email: "ada@example.com", DisplayName: "Ada", Status: "active"}}}
	publisher := &fakePublisher{}
	token := base64.RawURLEncoding.EncodeToString([]byte("secret-verification-token"))
	if err := New(repo, publisher).Verify(context.Background(), Input{Token: token}); err != nil {
		t.Fatalf("Verify() error = %v", err)
	}
	encoded, err := json.Marshal(publisher.event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	if strings.Contains(string(encoded), token) {
		t.Fatalf("email verified event exposes token: %s", encoded)
	}
}
