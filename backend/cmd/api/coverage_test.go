package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/refresh"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type healthcheckDoer struct {
	response *http.Response
	err      error
	requests int
}

func (d *healthcheckDoer) Do(*http.Request) (*http.Response, error) {
	d.requests++
	return d.response, d.err
}

func TestRNF005_HealthcheckRejectsInvalidURLAndTransportFailure(t *testing.T) {
	t.Run("invalid URL does not call transport", func(t *testing.T) {
		doer := &healthcheckDoer{}
		if got := runHealthcheck(doer, "://not-a-url"); got != 1 {
			t.Fatalf("runHealthcheck() = %d, want 1", got)
		}
		if doer.requests != 0 {
			t.Fatalf("transport calls = %d, want 0", doer.requests)
		}
	})

	t.Run("transport error is unhealthy", func(t *testing.T) {
		doer := &healthcheckDoer{err: errors.New("connection failed")}
		if got := runHealthcheck(doer, "http://health.test/healthz"); got != 1 {
			t.Fatalf("runHealthcheck() = %d, want 1", got)
		}
	})
}

func TestRNF005_HealthcheckURLUsesDefaultOrConfiguredPort(t *testing.T) {
	t.Setenv("API_PORT", "")
	if got := healthcheckURL(); got != "http://127.0.0.1:8081/healthz" {
		t.Fatalf("healthcheckURL() = %q", got)
	}

	t.Setenv("API_PORT", "9123")
	if got := healthcheckURL(); got != "http://127.0.0.1:9123/healthz" {
		t.Fatalf("healthcheckURL() = %q", got)
	}
}

func TestRNF005_HealthcheckClosesSuccessfulResponse(t *testing.T) {
	body := &trackingReadCloser{Reader: strings.NewReader("ok")}
	doer := &healthcheckDoer{response: &http.Response{StatusCode: http.StatusOK, Body: body}}
	if got := runHealthcheck(doer, "http://health.test/healthz"); got != 0 {
		t.Fatalf("runHealthcheck() = %d, want 0", got)
	}
	if !body.closed {
		t.Fatal("healthcheck response body was not closed")
	}
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

func TestRNF005_LoginSecurityPublisherBuildsCompleteNotification(t *testing.T) {
	userID := uuid.New()
	lockedUntil := time.Date(2026, 9, 26, 10, 30, 0, 0, time.UTC)
	address := netip.MustParseAddr("203.0.113.10")
	publisher := &adapterPublisher{}
	adapter := loginSecurityEventPublisher{
		users:     adapterUsers{user: store.User{ID: userID, Email: "user@example.test", DisplayName: "User"}},
		publisher: publisher,
	}
	ctx := context.Background()
	err := adapter.PublishSecurityEvent(ctx, login.SecurityEvent{
		Type: events.TypeAccountLocked, UserID: userID, LockedUntil: lockedUntil, FailedAttempts: 4, IP: &address,
	})
	if err != nil {
		t.Fatalf("PublishSecurityEvent() error = %v", err)
	}
	if len(publisher.events) != 1 || publisher.events[0].routingKey != events.TypeAccountLocked {
		t.Fatalf("published events = %#v", publisher.events)
	}
	message, ok := publisher.events[0].body.(events.AccountLocked)
	if !ok {
		t.Fatalf("message type = %T", publisher.events[0].body)
	}
	if message.EventType != events.TypeAccountLocked || message.TraceID != "" || message.Data.UserID != userID || message.Data.Email != "user@example.test" || message.Data.DisplayName != "User" || !message.Data.LockedUntil.Equal(lockedUntil) || message.Data.FailedAttempts != 4 || message.Data.IP != address.String() {
		t.Fatalf("notification = %#v", message)
	}
}

func TestRNF005_SecurityPublishersHandleLookupAndPublishFailures(t *testing.T) {
	userID := uuid.New()
	publishErr := errors.New("broker unavailable")
	for _, tt := range []struct {
		name    string
		publish func(loginSecurityEventPublisher, refreshSecurityEventPublisher) error
	}{
		{
			name: "login publisher returns broker failure",
			publish: func(loginPublisher loginSecurityEventPublisher, _ refreshSecurityEventPublisher) error {
				return loginPublisher.PublishSecurityEvent(context.Background(), login.SecurityEvent{Type: events.TypeAccountLocked, UserID: userID})
			},
		},
		{
			name: "refresh publisher returns broker failure",
			publish: func(_ loginSecurityEventPublisher, refreshPublisher refreshSecurityEventPublisher) error {
				return refreshPublisher.PublishSecurityEvent(context.Background(), refresh.SecurityEvent{Type: events.TypeRefreshReuseDetected, UserID: userID})
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			publisher := &failingAdapterPublisher{err: publishErr}
			loginPublisher := loginSecurityEventPublisher{users: adapterUsers{user: store.User{ID: userID}}, publisher: publisher}
			refreshPublisher := refreshSecurityEventPublisher{users: adapterUsers{user: store.User{ID: userID}}, publisher: publisher}
			if err := tt.publish(loginPublisher, refreshPublisher); !errors.Is(err, publishErr) {
				t.Fatalf("PublishSecurityEvent() error = %v, want %v", err, publishErr)
			}
		})
	}

	missing := loginSecurityEventPublisher{users: adapterUsers{err: errors.New("missing user")}, publisher: &adapterPublisher{}}
	if err := missing.PublishSecurityEvent(context.Background(), login.SecurityEvent{UserID: userID}); err != nil {
		t.Fatalf("missing recipient error = %v", err)
	}
}

func TestRNF005_RefreshSecurityPublisherBuildsCompleteNotification(t *testing.T) {
	userID, familyID := uuid.New(), uuid.New()
	address := netip.MustParseAddr("2001:db8::7")
	publisher := &adapterPublisher{}
	adapter := refreshSecurityEventPublisher{
		users:     adapterUsers{user: store.User{ID: userID, Email: "user@example.test", DisplayName: "User"}},
		publisher: publisher,
	}
	ctx := context.Background()
	if err := adapter.PublishSecurityEvent(ctx, refresh.SecurityEvent{Type: events.TypeRefreshReuseDetected, UserID: userID, FamilyID: familyID, RevokedCount: 3, IP: &address}); err != nil {
		t.Fatalf("PublishSecurityEvent() error = %v", err)
	}
	message, ok := publisher.events[0].body.(refreshReuseNotification)
	if !ok {
		t.Fatalf("message type = %T", publisher.events[0].body)
	}
	if message.EventType != events.TypeRefreshReuseDetected || message.TraceID != "" || message.Data.UserID != userID || message.Data.FamilyID != familyID || message.Data.Email != "user@example.test" || message.Data.DisplayName != "User" || message.Data.RevokedCount != 3 || message.Data.IP != address.String() {
		t.Fatalf("notification = %#v", message)
	}
}

func TestRNF005_AddressStringHandlesMissingAndPresentAddresses(t *testing.T) {
	if got := addressString(nil); got != "" {
		t.Fatalf("addressString(nil) = %q", got)
	}
	address := netip.MustParseAddr("198.51.100.8")
	if got := addressString(&address); got != "198.51.100.8" {
		t.Fatalf("addressString() = %q", got)
	}
}

type failingAdapterPublisher struct{ err error }

func (p *failingAdapterPublisher) Publish(context.Context, string, any) error { return p.err }
