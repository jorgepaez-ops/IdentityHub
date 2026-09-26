package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/refresh"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type adapterUsers struct {
	user store.User
	err  error
}

func (u adapterUsers) GetUserByID(context.Context, uuid.UUID) (store.User, error) {
	return u.user, u.err
}

type publishedEvent struct {
	routingKey string
	body       any
}
type adapterPublisher struct{ events []publishedEvent }

func (p *adapterPublisher) Publish(_ context.Context, routingKey string, body any) error {
	p.events = append(p.events, publishedEvent{routingKey, body})
	return nil
}

func TestRF017_LoginSecurityEventPublisherPublishesRecipient(t *testing.T) {
	userID := uuid.New()
	publisher := &adapterPublisher{}
	adapter := loginSecurityEventPublisher{users: adapterUsers{user: store.User{ID: userID, Email: "ada@example.com", DisplayName: "Ada"}}, publisher: publisher}
	lockedUntil := time.Date(2026, 9, 25, 12, 30, 0, 0, time.UTC)
	if err := adapter.PublishSecurityEvent(context.Background(), login.SecurityEvent{Type: events.TypeAccountLocked, UserID: userID, LockedUntil: lockedUntil, FailedAttempts: 5}); err != nil {
		t.Fatalf("PublishSecurityEvent() error = %v", err)
	}
	if len(publisher.events) != 1 || publisher.events[0].routingKey != events.TypeAccountLocked {
		t.Fatalf("published events = %#v", publisher.events)
	}
	body, err := json.Marshal(publisher.events[0].body)
	if err != nil {
		t.Fatal(err)
	}
	// The AsyncAPI contract (specs/04-events/asyncapi.yaml, AccountLocked
	// message) requires lockedUntil and failedAttempts, not just the
	// recipient fields — a prior version of this adapter hardcoded them to
	// zero values and this test didn't notice.
	var got struct {
		Data struct {
			Email          string    `json:"email"`
			DisplayName    string    `json:"displayName"`
			LockedUntil    time.Time `json:"lockedUntil"`
			FailedAttempts int       `json:"failedAttempts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Data.Email != "ada@example.com" || got.Data.DisplayName != "Ada" {
		t.Fatalf("recipient payload = %#v", got.Data)
	}
	if !got.Data.LockedUntil.Equal(lockedUntil) || got.Data.FailedAttempts != 5 {
		t.Fatalf("contract-required lockout fields = %#v", got.Data)
	}
}

// TestRNF004_HealthcheckSubcommandReportsHealthyServer covers the exec-form
// healthcheck T27 needs: distroless has no shell, so the compose healthcheck
// can no longer run `curl`, and must instead invoke the binary itself.
func TestRNF004_HealthcheckSubcommandReportsHealthyServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if got := runHealthcheck(srv.Client(), srv.URL+"/healthz"); got != 0 {
		t.Fatalf("runHealthcheck() = %d, want 0 for a healthy server", got)
	}
}

func TestRNF004_HealthcheckSubcommandReportsUnhealthyServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if got := runHealthcheck(srv.Client(), srv.URL+"/healthz"); got != 1 {
		t.Fatalf("runHealthcheck() = %d, want 1 for an unhealthy server", got)
	}
}

func TestRNF004_HealthcheckSubcommandReportsConnectionFailure(t *testing.T) {
	if got := runHealthcheck(http.DefaultClient, "http://127.0.0.1:1/healthz"); got != 1 {
		t.Fatalf("runHealthcheck() = %d, want 1 when the server is unreachable", got)
	}
}

func TestRF006_RefreshSecurityEventPublisherSkipsMissingUser(t *testing.T) {
	publisher := &adapterPublisher{}
	adapter := refreshSecurityEventPublisher{users: adapterUsers{err: errors.New("not found")}, publisher: publisher}
	if err := adapter.PublishSecurityEvent(context.Background(), refresh.SecurityEvent{Type: events.TypeRefreshReuseDetected, UserID: uuid.New()}); err != nil {
		t.Fatalf("PublishSecurityEvent() error = %v", err)
	}
	if len(publisher.events) != 0 {
		t.Fatalf("published events = %#v, want none", publisher.events)
	}
}
