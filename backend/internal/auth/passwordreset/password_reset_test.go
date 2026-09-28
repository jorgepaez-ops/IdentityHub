package passwordreset

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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
	inConfirmation      bool
}
type resetFakeWriter struct {
	consumes, revocations int
	token                 store.CreatePasswordResetTokenParams
	consumeErr            error
	createForEmailCalls   int
	createForEmail        string
	unlocked              bool
	auditEvents           []store.InsertAuditEventParams
	auditErr              error
}

func (r *resetFakeRepository) PasswordResetTokenIsUsable(context.Context, []byte) (bool, error) {
	return r.usable, r.precheckErr
}
func (r *resetFakeRepository) WithinPasswordResetRequestTransaction(_ context.Context, fn func(store.PasswordResetRequestWriter) error) error {
	r.requestTransactions++
	return fn(r.writer)
}
func (r *resetFakeRepository) WithinPasswordResetConfirmationTransaction(_ context.Context, fn func(store.PasswordResetConfirmationWriter) error) error {
	r.inConfirmation = true
	defer func() { r.inConfirmation = false }()
	return fn(r.writer)
}
func (w *resetFakeWriter) CreatePasswordResetTokenForEmail(_ context.Context, email string, params store.CreatePasswordResetTokenParams) (bool, error) {
	w.createForEmailCalls++
	w.createForEmail = email
	w.token = params
	return email != "absent@example.test", nil
}

func (w *resetFakeWriter) ConsumePasswordResetTokenAndRevokeSessions(context.Context, store.ConsumePasswordResetTokenParams) (store.User, bool, error) {
	w.consumes++
	if w.consumeErr != nil {
		return store.User{}, false, w.consumeErr
	}
	w.revocations++
	return store.User{ID: uuid.New()}, w.unlocked, nil
}

func (w *resetFakeWriter) InsertAuditEvent(_ context.Context, params store.InsertAuditEventParams) (store.AuditEvent, error) {
	w.auditEvents = append(w.auditEvents, params)
	if w.auditErr != nil {
		return store.AuditEvent{}, w.auditErr
	}
	return store.AuditEvent{Action: params.Action}, nil
}

type resetFakeHasher struct{ calls int }

func (h *resetFakeHasher) Hash(string) (string, error) { h.calls++; return "$argon2id$test", nil }

type resetFakePublisher struct {
	events int
	event  any
	err    error
	// onPublish observes the moment of publication.
	onPublish func()
}

func (p *resetFakePublisher) Publish(_ context.Context, _ string, event any) error {
	if p.onPublish != nil {
		p.onPublish()
	}
	p.events++
	p.event = event
	return p.err
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

// The published event deliberately carries accountExists (a state bit) so the
// worker can skip SMTP for a missing account (AM-004); what it must never
// carry is the user's identity, so this only checks for that.
func TestRF015_SolicitudNoIncluyeElUserIdEnElEvento(t *testing.T) {
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
	if bytes.Contains(payload, []byte(`"userId"`)) {
		t.Fatalf("event leaks the user id: %s", payload)
	}
}

func TestRF015_TokenVacioDevuelveErrTokenRequired(t *testing.T) {
	repo := &resetFakeRepository{writer: &resetFakeWriter{}}
	err := New(repo, &resetFakePublisher{}, &resetFakeHasher{}, bytes.NewReader(make([]byte, 32)), nil).Confirm(context.Background(), "  ", "correct horse battery")
	if !errors.Is(err, ErrTokenRequired) {
		t.Fatalf("err=%v want ErrTokenRequired", err)
	}
}

// D13: every completed reset records password_reset_completed on the acting
// user, with metadata reporting whether it also cleared an RF-017 lockout.
func TestRF015_ConfirmarRegistraAuditoriaConEstadoDeDesbloqueo(t *testing.T) {
	raw := []byte("valid-password-reset-token")
	cases := []struct {
		name     string
		unlocked bool
	}{
		{"cuenta bloqueada", true},
		{"cuenta activa", false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			writer := &resetFakeWriter{unlocked: tt.unlocked}
			repo := &resetFakeRepository{usable: true, writer: writer}
			err := New(repo, &resetFakePublisher{}, &resetFakeHasher{}, bytes.NewReader(make([]byte, 32)), nil).Confirm(context.Background(), base64.RawURLEncoding.EncodeToString(raw), "correct horse battery")
			if err != nil {
				t.Fatal(err)
			}
			if len(writer.auditEvents) != 1 {
				t.Fatalf("audit events=%d want 1", len(writer.auditEvents))
			}
			event := writer.auditEvents[0]
			if event.Action != "password_reset_completed" {
				t.Fatalf("action=%q want password_reset_completed", event.Action)
			}
			if event.ResourceType == nil || *event.ResourceType != "user" {
				t.Fatalf("resourceType=%v want user", event.ResourceType)
			}
			var metadata struct {
				Unlocked bool `json:"unlocked"`
			}
			if err := json.Unmarshal(event.Metadata, &metadata); err != nil {
				t.Fatalf("unmarshal metadata: %v", err)
			}
			if metadata.Unlocked != tt.unlocked {
				t.Fatalf("metadata unlocked=%t want %t", metadata.Unlocked, tt.unlocked)
			}
		})
	}
}

func TestRF015_AuditoriaFallidaNoSeIgnora(t *testing.T) {
	raw := []byte("valid-password-reset-token")
	writer := &resetFakeWriter{auditErr: errors.New("audit unavailable")}
	repo := &resetFakeRepository{usable: true, writer: writer}
	err := New(repo, &resetFakePublisher{}, &resetFakeHasher{}, bytes.NewReader(make([]byte, 32)), nil).Confirm(context.Background(), base64.RawURLEncoding.EncodeToString(raw), "correct horse battery")
	if err == nil {
		t.Fatal("want an error when the audit insert fails")
	}
}

// D16: the D14 alert leaves only after the reset has committed, and a broker
// failure at that point neither undoes the reset nor changes its result.
func TestRF015_ElAvisoDeRestablecimientoSePublicaDespuesDelCommit(t *testing.T) {
	raw := []byte("valid-password-reset-token")
	repo := &resetFakeRepository{usable: true, writer: &resetFakeWriter{unlocked: true}}
	var openAtPublish []bool
	publisher := &resetFakePublisher{onPublish: func() { openAtPublish = append(openAtPublish, repo.inConfirmation) }}
	err := New(repo, publisher, &resetFakeHasher{}, bytes.NewReader(make([]byte, 32)), nil).Confirm(context.Background(), base64.RawURLEncoding.EncodeToString(raw), "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if len(openAtPublish) != 1 || openAtPublish[0] {
		t.Fatalf("transaction open at publish = %v, want one publish after commit", openAtPublish)
	}
	event, ok := publisher.event.(events.PasswordResetCompleted)
	if !ok || !event.Data.Unlocked || event.Data.UserID == uuid.Nil {
		t.Fatalf("published event=%+v", publisher.event)
	}
}

func TestRF015_FallaPublicarElAvisoNoDeshaceElRestablecimientoYQuedaEnElLog(t *testing.T) {
	raw := []byte("valid-password-reset-token")
	writer := &resetFakeWriter{}
	repo := &resetFakeRepository{usable: true, writer: writer}
	var logs bytes.Buffer
	service := New(repo, &resetFakePublisher{err: errors.New("broker unavailable")}, &resetFakeHasher{}, bytes.NewReader(make([]byte, 32)), nil).WithLogger(slog.New(slog.NewTextHandler(&logs, nil)))

	if err := service.Confirm(context.Background(), base64.RawURLEncoding.EncodeToString(raw), "correct horse battery"); err != nil {
		t.Fatalf("Confirm() = %v, want success: the reset is already committed", err)
	}
	if writer.consumes != 1 || len(writer.auditEvents) != 1 {
		t.Fatalf("consumes=%d audits=%d, want the reset kept", writer.consumes, len(writer.auditEvents))
	}
	line := logs.String()
	if !strings.Contains(line, "user_id=") || !strings.Contains(line, "broker unavailable") || strings.Contains(line, "correct horse battery") {
		t.Fatalf("log=%q, want the user id and the error, no secrets", line)
	}
}

func TestRF015_SinCommitNoSePublicaElAviso(t *testing.T) {
	writer := &resetFakeWriter{auditErr: errors.New("audit unavailable")}
	publisher := &resetFakePublisher{}
	repo := &resetFakeRepository{usable: true, writer: writer}
	err := New(repo, publisher, &resetFakeHasher{}, bytes.NewReader(make([]byte, 32)), nil).Confirm(context.Background(), base64.RawURLEncoding.EncodeToString([]byte("valid-password-reset-token")), "correct horse battery")
	if err == nil || publisher.events != 0 {
		t.Fatalf("err=%v published=%d, want a failed reset that mails nothing", err, publisher.events)
	}
}
