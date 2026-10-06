package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/smtp"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/jorgepaez/identity-hub/internal/config"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/notify"
)

// fakeAcknowledger records Ack/Nack/Reject calls so handle() can be exercised
// without a real broker (amqp.Delivery.Acknowledger is built for this).
type fakeAcknowledger struct {
	mu               sync.Mutex
	acks, nacks, rej int
}

func (f *fakeAcknowledger) Ack(uint64, bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.acks++
	return nil
}
func (f *fakeAcknowledger) Nack(uint64, bool, bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nacks++
	return nil
}
func (f *fakeAcknowledger) Reject(uint64, bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rej++
	return nil
}

func TestRF012_FalloTransitorioReintentaSinPerderLaNotificacion(t *testing.T) {
	event := events.UserRegistered{Envelope: events.NewEnvelope(events.TypeUserRegistered, "")}
	event.Data.Email = "ana@example.com"
	event.Data.DisplayName = "Ana"
	event.Data.VerificationToken = "token"
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}

	var logBuf bytes.Buffer
	w := &worker{
		logger: slog.New(slog.NewTextHandler(&logBuf, nil)),
		// Puerto reservado sin servicio escuchando: la entrega falla rápido y
		// de forma determinista, sin red real.
		cfg:  &config.Config{SMTPHost: "127.0.0.1", SMTPPort: 1, SMTPFrom: "no-reply@identity.local", PublicBaseURL: "http://localhost:8080"},
		seen: make(map[string]time.Time),
	}

	ack := &fakeAcknowledger{}
	delivery := amqp.Delivery{Body: body, DeliveryTag: 1, Acknowledger: ack}

	w.handle(context.Background(), delivery)
	w.handle(context.Background(), delivery)

	if ack.nacks != 2 {
		t.Errorf("nacks = %d, want 2 (both attempts must retry after the transient SMTP failure)", ack.nacks)
	}
	if ack.acks != 0 {
		t.Errorf("acks = %d, want 0 (a failed delivery must never be acknowledged as done)", ack.acks)
	}
	if strings.Contains(logBuf.String(), "evento duplicado") {
		t.Errorf("second attempt was treated as a duplicate instead of being retried: %s", logBuf.String())
	}
}

func TestRF012_ConstruyeElMensajeSMTPConAsuntoYCuerpoRenderizados(t *testing.T) {
	message := notify.Message{
		To:      "ana@example.com",
		Subject: "Verify your Identity Hub email",
		Body:    "Hello Ana,\n\nVerify your email address: https://id.example/verify-email?token=abc\n",
	}

	raw := string(buildRawMessage("no-reply@identity.local", message))

	if !strings.Contains(raw, "From: no-reply@identity.local") {
		t.Errorf("raw message missing From header: %s", raw)
	}
	if !strings.Contains(raw, "To: ana@example.com") {
		t.Errorf("raw message missing To header: %s", raw)
	}
	if !strings.Contains(raw, "Subject: [Identity Hub] Verify your Identity Hub email") {
		t.Errorf("raw message missing prefixed Subject header: %s", raw)
	}
	_, encodedBody, found := strings.Cut(raw, "\r\n\r\n")
	if !found {
		t.Fatalf("raw message has no header/body separator: %s", raw)
	}
	if encodedBody != message.Body {
		t.Errorf("body = %q, want %q", encodedBody, message.Body)
	}
}

func TestRF012_MensajeSMTPDeclaraMIMEYCodificaContenidoUTF8(t *testing.T) {
	message, err := notify.Render(events.TypeUserRegistered, "https://id.example", []byte(`{
		"data": {
			"email": "ana@example.com",
			"displayName": "Jos\u00e9 \u674e",
			"verificationToken": "abc"
		}
	}`))
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	message.Subject = "Bienvenida para Zo\u00eb"

	raw := string(buildRawMessage("no-reply@identity.local", message))
	headers, encodedBody, found := strings.Cut(raw, "\r\n\r\n")
	if !found {
		t.Fatal("raw message has no header/body separator")
	}
	if !strings.Contains(headers, "MIME-Version: 1.0\r\n") {
		t.Fatalf("raw message is missing MIME-Version: %s", raw)
	}
	if !strings.Contains(headers, "Content-Type: text/plain; charset=utf-8\r\n") {
		t.Fatalf("raw message is missing UTF-8 content type: %s", raw)
	}
	if !strings.Contains(headers, "Content-Transfer-Encoding: 8bit\r\n") {
		t.Fatalf("raw message is missing 8bit transfer encoding: %s", raw)
	}

	var encodedSubject string
	for _, header := range strings.Split(headers, "\r\n") {
		if strings.HasPrefix(header, "Subject: ") {
			encodedSubject = strings.TrimPrefix(header, "Subject: ")
			break
		}
	}
	if encodedSubject == "" {
		t.Fatal("raw message is missing Subject header")
	}
	decodedSubject, err := new(mime.WordDecoder).DecodeHeader(encodedSubject)
	if err != nil {
		t.Fatalf("DecodeHeader(%q): %v", encodedSubject, err)
	}
	if want := "[Identity Hub] " + message.Subject; decodedSubject != want {
		t.Errorf("decoded subject = %q, want %q", decodedSubject, want)
	}

	if encodedBody != message.Body {
		t.Errorf("body = %q, want %q", encodedBody, message.Body)
	}
}

func TestRF012_MensajeSMTPMantieneAsuntoASCIIVisible(t *testing.T) {
	message := notify.Message{To: "ana@example.com", Subject: "Verify your Identity Hub email", Body: "Hello Ana."}
	raw := string(buildRawMessage("no-reply@identity.local", message))

	if !strings.Contains(raw, "Subject: [Identity Hub] Verify your Identity Hub email\r\n") {
		t.Errorf("ASCII subject was unexpectedly encoded: %s", raw)
	}
}

func TestRF012_MensajeSMTPNoPermiteInyeccionDeCabeceras(t *testing.T) {
	raw := string(buildRawMessage("no-reply@identity.local", notify.Message{
		To:      "ana@example.com\r\nBcc: attacker@example.com",
		Subject: "Welcome\r\nBcc: attacker@example.com",
		Body:    "Hello.",
	}))
	if strings.Contains(raw, "\r\nBcc:") {
		t.Fatalf("raw message contains injected Bcc header: %q", raw)
	}

	_, err := notify.Render(events.TypeUserRegistered, "https://id.example", []byte(`{
		"data": {
			"email": "ana@example.com\r\nBcc: attacker@example.com",
			"verificationToken": "abc"
		}
	}`))
	if err == nil {
		t.Fatal("Render accepted a recipient with CR/LF")
	}
}

type smtpCall struct {
	address string
	from    string
	to      []string
	body    []byte
}

func TestRNF005_DeliveryUsesInjectedSMTPTransportAfterRendering(t *testing.T) {
	event := events.UserRegistered{Envelope: events.NewEnvelope(events.TypeUserRegistered, "")}
	event.Data.Email = "recipient@example.test"
	event.Data.DisplayName = "Recipient"
	event.Data.VerificationToken = "verification-token"
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	var call smtpCall
	w := &worker{
		cfg: &config.Config{SMTPHost: "smtp.test", SMTPPort: 2525, SMTPFrom: "sender@example.test", PublicBaseURL: "https://identity.example.test"},
		sendMail: func(address string, _ smtp.Auth, from string, to []string, message []byte) error {
			call = smtpCall{address: address, from: from, to: to, body: message}
			return nil
		},
	}
	if err := w.deliver(context.Background(), event.Envelope, body); err != nil {
		t.Fatalf("deliver() error = %v", err)
	}
	if call.address != "smtp.test:2525" || call.from != "sender@example.test" || !reflect.DeepEqual(call.to, []string{"recipient@example.test"}) {
		t.Fatalf("SMTP call = %#v", call)
	}
	if !strings.Contains(string(call.body), "Subject: [Identity Hub]") || !strings.Contains(string(call.body), "recipient@example.test") {
		t.Fatalf("SMTP body = %s", call.body)
	}
}

func TestRNF005_DeliveryStopsBeforeSMTPWhenContextIsCanceled(t *testing.T) {
	event := events.UserRegistered{Envelope: events.NewEnvelope(events.TypeUserRegistered, "")}
	event.Data.Email = "recipient@example.test"
	event.Data.DisplayName = "Recipient"
	event.Data.VerificationToken = "verification-token"
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	called := false
	w := &worker{
		cfg: &config.Config{SMTPHost: "smtp.test", SMTPPort: 2525, SMTPFrom: "sender@example.test", PublicBaseURL: "https://identity.example.test"},
		sendMail: func(string, smtp.Auth, string, []string, []byte) error {
			called = true
			return nil
		},
	}
	if err := w.deliver(ctx, event.Envelope, body); !errors.Is(err, context.Canceled) {
		t.Fatalf("deliver() error = %v, want context canceled", err)
	}
	if called {
		t.Fatal("SMTP transport was called after cancellation")
	}
}

func TestRF015_EntregaDeCuentaAusenteNoEnviaCorreo(t *testing.T) {
	event := events.PasswordResetRequested{Envelope: events.NewEnvelope(events.TypePasswordResetRequested, "")}
	event.Data.Email = "absent@example.test"
	event.Data.ResetToken = "non-consumable-token"
	event.Data.AccountExists = false
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	called := false
	w := &worker{
		cfg: &config.Config{SMTPHost: "smtp.test", SMTPPort: 2525, SMTPFrom: "sender@example.test", PublicBaseURL: "https://identity.example.test"},
		sendMail: func(string, smtp.Auth, string, []string, []byte) error {
			called = true
			return nil
		},
	}
	if err := w.deliver(context.Background(), event.Envelope, body); err != nil {
		t.Fatalf("deliver() error = %v", err)
	}
	if called {
		t.Fatal("SMTP transport was called for a missing account")
	}
}

// The anti-enumeration skip (AM-004) must still leave a trace for operators,
// but never the email address or the reset token (RNF-012).
func TestRF015_EntregaDeCuentaAusenteRegistraElDescarteSinDatosSensibles(t *testing.T) {
	event := events.PasswordResetRequested{Envelope: events.NewEnvelope(events.TypePasswordResetRequested, "")}
	event.Data.Email = "absent-log@example.test"
	event.Data.ResetToken = "non-consumable-token-for-log"
	event.Data.AccountExists = false
	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	var logBuf bytes.Buffer
	w := &worker{
		logger: slog.New(slog.NewTextHandler(&logBuf, nil)),
		cfg:    &config.Config{SMTPHost: "smtp.test", SMTPPort: 2525, SMTPFrom: "sender@example.test", PublicBaseURL: "https://identity.example.test"},
		sendMail: func(string, smtp.Auth, string, []string, []byte) error {
			t.Fatal("SMTP transport was called for a missing account")
			return nil
		},
	}
	if err := w.deliver(context.Background(), event.Envelope, body); err != nil {
		t.Fatalf("deliver() error = %v", err)
	}
	logged := logBuf.String()
	if logged == "" {
		t.Fatal("skipping a missing-account reset left no trace in the logs")
	}
	if strings.Contains(logged, event.Data.Email) {
		t.Fatalf("skip log leaked the email address: %s", logged)
	}
	if strings.Contains(logged, event.Data.ResetToken) {
		t.Fatalf("skip log leaked the reset token: %s", logged)
	}
}
