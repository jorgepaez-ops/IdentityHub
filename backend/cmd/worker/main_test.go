package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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
	if !strings.Contains(raw, message.Body) {
		t.Errorf("raw message missing rendered body: %s", raw)
	}
}
