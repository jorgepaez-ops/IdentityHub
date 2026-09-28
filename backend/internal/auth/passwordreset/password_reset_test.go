package passwordreset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type resetCountingReader struct {
	reader *bytes.Reader
	reads  int
}

func (r *resetCountingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.reads += n
	return n, err
}

type resetFakeRepository struct {
	usable              bool
	precheckErr         error
	writer              *resetFakeWriter
	requestTransactions int
}
type resetFakeWriter struct {
	consumes, revocations int
	token                 store.CreatePasswordResetTokenParams
	consumeErr            error
	createForEmailCalls   int
	createForEmail        string
}

func (r *resetFakeRepository) PasswordResetTokenIsUsable(context.Context, []byte) (bool, error) {
	return r.usable, r.precheckErr
}
func (r *resetFakeRepository) WithinPasswordResetRequestTransaction(_ context.Context, fn func(store.PasswordResetRequestWriter) error) error {
	r.requestTransactions++
	return fn(r.writer)
}
func (r *resetFakeRepository) WithinPasswordResetConfirmationTransaction(_ context.Context, fn func(store.PasswordResetConfirmationWriter) error) error {
	return fn(r.writer)
}
func (w *resetFakeWriter) CreatePasswordResetTokenForEmail(_ context.Context, email string, params store.CreatePasswordResetTokenParams) (bool, error) {
	w.createForEmailCalls++
	w.createForEmail = email
	w.token = params
	return email != "absent@example.test", nil
}

func (w *resetFakeWriter) ConsumePasswordResetTokenAndRevokeSessions(context.Context, store.ConsumePasswordResetTokenParams) (store.User, error) {
	w.consumes++
	if w.consumeErr != nil {
		return store.User{}, w.consumeErr
	}
	w.revocations++
	return store.User{ID: uuid.New()}, nil
}

type resetFakeHasher struct{ calls int }

func (h *resetFakeHasher) Hash(string) (string, error) { h.calls++; return "$argon2id$test", nil }

type resetFakePublisher struct {
	events int
	event  any
}

func (p *resetFakePublisher) Publish(_ context.Context, _ string, event any) error {
	p.events++
	p.event = event
	return nil
}

func TestRF015_SolicitudNoEnumeraYHaceTrabajoComparable(t *testing.T) {
	raw := bytes.Repeat([]byte{1}, 64)
	cases := []struct {
		name       string
		email      string
		wantEvents int
	}{
		{"cuenta existente", "Ada@Example.Test", 1}, {"cuenta ausente", "absent@example.test", 1},
	}
	var reads, transactions, unifiedWrites []int
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			repo := &resetFakeRepository{writer: &resetFakeWriter{}}
			publisher := &resetFakePublisher{}
			random := &resetCountingReader{reader: bytes.NewReader(raw)}
			service := New(repo, publisher, &resetFakeHasher{}, random, nil)
			if err := service.Request(context.Background(), tt.email); err != nil {
				t.Fatal(err)
			}
			if publisher.events != tt.wantEvents {
				t.Fatalf("events=%d want %d", publisher.events, tt.wantEvents)
			}
			reads = append(reads, random.reads)
			transactions = append(transactions, repo.requestTransactions)
			unifiedWrites = append(unifiedWrites, repo.writer.createForEmailCalls)
		})
	}
	if reads[0] != reads[1] || reads[0] != 32 {
		t.Fatalf("entropy work = %v, want identical 32-byte work", reads)
	}
	if transactions[0] != 1 || transactions[1] != 1 || unifiedWrites[0] != 1 || unifiedWrites[1] != 1 {
		t.Fatalf("transaction coverage transactions=%v unifiedWrites=%v, want [1 1]", transactions, unifiedWrites)
	}
}

func TestRF015_EventoDeSolicitudReflejaLaExistenciaSoloParaElBroker(t *testing.T) {
	cases := []struct {
		name, email   string
		accountExists bool
	}{
		{name: "cuenta existente", email: "ada@example.test", accountExists: true},
		{name: "cuenta ausente", email: "absent@example.test", accountExists: false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			publisher := &resetFakePublisher{}
			repo := &resetFakeRepository{writer: &resetFakeWriter{}}
			service := New(repo, publisher, &resetFakeHasher{}, bytes.NewReader(bytes.Repeat([]byte{4}, 32)), nil)
			if err := service.Request(context.Background(), tt.email); err != nil {
				t.Fatal(err)
			}
			event, ok := publisher.event.(events.PasswordResetRequested)
			if !ok {
				t.Fatalf("published event type=%T", publisher.event)
			}
			if event.Data.AccountExists != tt.accountExists {
				t.Fatalf("accountExists=%t want %t", event.Data.AccountExists, tt.accountExists)
			}
		})
	}
}
func TestRF015_TokenInvalidoNoCalculaHash(t *testing.T) {
	h := &resetFakeHasher{}
	repo := &resetFakeRepository{usable: false, writer: &resetFakeWriter{}}
	err := New(repo, &resetFakePublisher{}, h, bytes.NewReader(bytes.Repeat([]byte{2}, 32)), nil).Confirm(context.Background(), base64.RawURLEncoding.EncodeToString([]byte("invalid-reset-token")), "correct horse battery")
	if !errors.Is(err, ErrTokenInvalid) || h.calls != 0 {
		t.Fatalf("err=%v hashes=%d", err, h.calls)
	}
}
func TestRF015_ConfirmarConsumeUnaVezYRevocaSesiones(t *testing.T) {
	raw := []byte("valid-password-reset-token")
	writer := &resetFakeWriter{}
	repo := &resetFakeRepository{usable: true, writer: writer}
	err := New(repo, &resetFakePublisher{}, &resetFakeHasher{}, bytes.NewReader(make([]byte, 32)), nil).Confirm(context.Background(), base64.RawURLEncoding.EncodeToString(raw), "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if writer.consumes != 1 || writer.revocations != 1 {
		t.Fatalf("consumes=%d revocations=%d", writer.consumes, writer.revocations)
	}
	writer.consumeErr = pgx.ErrNoRows
	if err := New(repo, &resetFakePublisher{}, &resetFakeHasher{}, bytes.NewReader(make([]byte, 32)), nil).Confirm(context.Background(), base64.RawURLEncoding.EncodeToString(raw), "correct horse battery"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("second use err=%v", err)
	}
}

func TestRF015_TokenExpiradoNoCalculaHash(t *testing.T) {
	h := &resetFakeHasher{}
	repo := &resetFakeRepository{usable: false, writer: &resetFakeWriter{}}
	err := New(repo, &resetFakePublisher{}, h, bytes.NewReader(bytes.Repeat([]byte{3}, 32)), nil).Confirm(context.Background(), base64.RawURLEncoding.EncodeToString([]byte("expired-reset-token")), "correct horse battery")
	if !errors.Is(err, ErrTokenInvalid) || h.calls != 0 {
		t.Fatalf("err=%v hashes=%d", err, h.calls)
	}
}

func TestRF015_SolicitudAusenteHaceTransaccionDeCobertura(t *testing.T) {
	repo := &resetFakeRepository{writer: &resetFakeWriter{}}
	service := New(repo, &resetFakePublisher{}, &resetFakeHasher{}, bytes.NewReader(bytes.Repeat([]byte{7}, 32)), nil)
	if err := service.Request(context.Background(), "absent@example.test"); err != nil {
		t.Fatal(err)
	}
	if repo.requestTransactions != 1 || repo.writer.createForEmailCalls != 1 {
		t.Fatalf("transactions=%d unified=%d, want 1/1", repo.requestTransactions, repo.writer.createForEmailCalls)
	}
}

func TestRF015_ErrorDePrevalidacionSeEnvuelveUnaVez(t *testing.T) {
	cause := errors.New("database unavailable")
	repo := &resetFakeRepository{usable: false, precheckErr: fmt.Errorf("check password reset token: %w", cause), writer: &resetFakeWriter{}}
	err := New(repo, &resetFakePublisher{}, &resetFakeHasher{}, bytes.NewReader(bytes.Repeat([]byte{8}, 32)), nil).Confirm(context.Background(), base64.RawURLEncoding.EncodeToString([]byte("token")), "correct horse battery")
	if !errors.Is(err, cause) {
		t.Fatalf("err=%v", err)
	}
	if got, want := err.Error(), "check password reset token: database unavailable"; got != want {
		t.Fatalf("error=%q want %q", got, want)
	}
}

func TestRF015_SolicitudGuardaSoloHashYVenceEnUnaHora(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	raw := bytes.Repeat([]byte{9}, 32)
	writer := &resetFakeWriter{}
	repo := &resetFakeRepository{writer: writer}
	if err := New(repo, &resetFakePublisher{}, &resetFakeHasher{}, bytes.NewReader(raw), func() time.Time { return now }).Request(context.Background(), "ada@example.test"); err != nil {
		t.Fatal(err)
	}
	wantHash := sha256.Sum256(raw)
	if !bytes.Equal(writer.token.TokenHash, wantHash[:]) || bytes.Equal(writer.token.TokenHash, raw) {
		t.Fatalf("stored token is not only the SHA-256 hash")
	}
	if got, want := writer.token.ExpiresAt, now.Add(time.Hour); !got.Equal(want) {
		t.Fatalf("expiry=%s want=%s", got, want)
	}
}

func TestRF015_SolicitudNoIncluyeIdentidadDeCuentaEnEvento(t *testing.T) {
	raw := bytes.Repeat([]byte{6}, 32)
	publisher := &resetFakePublisher{}
	repo := &resetFakeRepository{writer: &resetFakeWriter{}}
	if err := New(repo, publisher, &resetFakeHasher{}, bytes.NewReader(raw), nil).Request(context.Background(), "absent@example.test"); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(publisher.event)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"found"`, `"userId"`} {
		if bytes.Contains(payload, []byte(forbidden)) {
			t.Fatalf("event leaks account state/identity: %s", payload)
		}
	}
}
