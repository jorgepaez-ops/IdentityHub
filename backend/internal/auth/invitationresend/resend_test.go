package invitationresend

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
	"testing"
	"time"
)

type fakeRepository struct{ writer *fakeWriter }
type fakeWriter struct {
	user                 store.User
	userErr              error
	invalidated, created int
	token                store.CreateInvitationTokenParams
	audit                store.InsertAuditEventParams
}

func (r *fakeRepository) WithinInvitationResendTransaction(_ context.Context, fn func(store.InvitationResendWriter) error) error {
	return fn(r.writer)
}
func (w *fakeWriter) GetUserByIDForUpdate(context.Context, uuid.UUID) (store.User, error) {
	return w.user, w.userErr
}
func (w *fakeWriter) InvalidateInvitationTokens(context.Context, uuid.UUID) error {
	w.invalidated++
	return nil
}
func (w *fakeWriter) CreateInvitationToken(_ context.Context, params store.CreateInvitationTokenParams) error {
	w.created++
	w.token = params
	return nil
}
func (w *fakeWriter) InsertAuditEvent(_ context.Context, params store.InsertAuditEventParams) (store.AuditEvent, error) {
	w.audit = params
	return store.AuditEvent{}, nil
}

type publisher struct {
	events int
	event  any
}

func (p *publisher) Publish(_ context.Context, _ string, event any) error {
	p.events++
	p.event = event
	return nil
}
func TestRF001_ReenvioInvalidaTokenAnteriorYPublicaUnoNuevo(t *testing.T) {
	w := &fakeWriter{user: store.User{ID: uuid.New(), Email: "pending@example.test", DisplayName: "Pending", Status: "pending_verification"}}
	p := &publisher{}
	err := New(&fakeRepository{writer: w}, p, bytes.NewReader(bytes.Repeat([]byte{1}, 32)), nil).Resend(context.Background(), Input{ActorUserID: uuid.New(), UserID: w.user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if w.invalidated != 1 || w.created != 1 || p.events != 1 {
		t.Fatalf("invalidated=%d created=%d events=%d", w.invalidated, w.created, p.events)
	}
}
func TestRF001_ReenvioRechazaCuentaActiva(t *testing.T) {
	w := &fakeWriter{user: store.User{ID: uuid.New(), Status: "active"}}
	err := New(&fakeRepository{writer: w}, &publisher{}, bytes.NewReader(make([]byte, 32)), nil).Resend(context.Background(), Input{ActorUserID: uuid.New(), UserID: w.user.ID})
	if !errors.Is(err, ErrAccountNotPending) {
		t.Fatalf("err=%v", err)
	}
	if w.created != 0 {
		t.Fatal("created token for active account")
	}
}
func TestRF001_ReenvioNoEncuentraCuenta(t *testing.T) {
	w := &fakeWriter{userErr: pgx.ErrNoRows}
	err := New(&fakeRepository{writer: w}, &publisher{}, bytes.NewReader(make([]byte, 32)), nil).Resend(context.Background(), Input{ActorUserID: uuid.New(), UserID: uuid.New()})
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestRF001_ReenvioVenceEn24HorasYAditaYPublicaContenido(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	actor := uuid.New()
	user := store.User{ID: uuid.New(), Email: "pending@example.test", DisplayName: "Pending", Status: "pending_verification"}
	writer := &fakeWriter{user: user}
	publisher := &publisher{}
	raw := bytes.Repeat([]byte{4}, 32)
	if err := New(&fakeRepository{writer: writer}, publisher, bytes.NewReader(raw), func() time.Time { return now }).Resend(context.Background(), Input{ActorUserID: actor, UserID: user.ID}); err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(raw)
	if !bytes.Equal(writer.token.TokenHash, wantHash[:]) || bytes.Equal(writer.token.TokenHash, raw) {
		t.Fatal("replacement invitation was not stored only as its hash")
	}
	if got, want := writer.token.ExpiresAt, now.Add(24*time.Hour); !got.Equal(want) {
		t.Fatalf("expiry=%s want=%s", got, want)
	}
	if writer.audit.Action != "invitation_resent" || writer.audit.ActorUserID == nil || *writer.audit.ActorUserID != actor {
		t.Fatalf("audit=%+v", writer.audit)
	}
	event, ok := publisher.event.(events.UserInvited)
	if !ok {
		t.Fatalf("event=%T", publisher.event)
	}
	if event.EventType != events.TypeUserInvited || event.Data.Email != user.Email || event.Data.InvitationToken == "" {
		t.Fatalf("event=%+v", event)
	}
}
