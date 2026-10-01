package main

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/jorgepaez/identity-hub/internal/auth/bootstrap"
)

type bootstrapEnsurerStub struct {
	calls   int
	outcome bootstrap.Outcome
	err     error
}

func (s *bootstrapEnsurerStub) Ensure(_ context.Context, _ string) (bootstrap.Outcome, error) {
	s.calls++
	return s.outcome, s.err
}

func TestBootstrapStartup_OptionalNoOpsYNoFiltraCorreo(t *testing.T) {
	for _, tt := range []struct {
		name    string
		email   string
		outcome bootstrap.Outcome
		err     error
		wantErr bool
		calls   int
		wantLog string
	}{
		{name: "empty does not touch database", email: "", calls: 0},
		{name: "reissued invitation continues", email: "first-admin@example.test", outcome: bootstrap.OutcomeInvitationReissued, calls: 1},
		{name: "pending live invitation continues", email: "first-admin@example.test", outcome: bootstrap.OutcomeInvitationPending, calls: 1},
		{name: "existing admin continues", email: "first-admin@example.test", outcome: bootstrap.OutcomeAdminExists, calls: 1},
		{name: "email conflict warns and continues", email: "first-admin@example.test", outcome: bootstrap.OutcomeEmailConflict, calls: 1},
		{name: "undelivered invitation warns and continues", email: "first-admin@example.test", outcome: bootstrap.OutcomeInvitationUndelivered, calls: 1, wantLog: "could not be delivered"},
		{name: "undelivered live invitation warns and continues", email: "first-admin@example.test", outcome: bootstrap.OutcomeInvitationUndeliveredLive, calls: 1, wantLog: "could not be delivered"},
		{name: "bootstrap failure stops startup", email: "first-admin@example.test", err: errors.New("database unavailable"), wantErr: true, calls: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer
			logger := slog.New(slog.NewTextHandler(&logs, nil))
			service := &bootstrapEnsurerStub{outcome: tt.outcome, err: tt.err}
			err := ensureBootstrapAdmin(context.Background(), logger, service, tt.email)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ensureBootstrapAdmin() error = %v, want error=%t", err, tt.wantErr)
			}
			if service.calls != tt.calls {
				t.Fatalf("Ensure calls = %d, want %d", service.calls, tt.calls)
			}
			if tt.wantLog != "" && !strings.Contains(logs.String(), tt.wantLog) {
				t.Fatalf("log = %q, want it to contain %q", logs.String(), tt.wantLog)
			}
			if strings.Contains(logs.String(), "first-admin@example.test") {
				t.Fatalf("bootstrap log leaked email: %s", logs.String())
			}
		})
	}
}
