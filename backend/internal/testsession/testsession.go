//go:build integration

// Package testsession signs a user in through the real password and email-MFA
// steps for integration tests, capturing the code the worker would mail.
package testsession

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jorgepaez/identity-hub/internal/auth/lockout"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/mfa"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

// Mailbox implements the broker publisher and remembers the latest MFA code
// and every published account-locked event.
type Mailbox struct {
	mu     sync.Mutex
	code   string
	locked []events.AccountLocked
}

func (m *Mailbox) Publish(_ context.Context, _ string, payload any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch event := payload.(type) {
	case events.MfaChallengeIssued:
		m.code = event.Data.Code
	case events.AccountLocked:
		m.locked = append(m.locked, event)
	}
	return nil
}

// Code returns the most recently mailed MFA code.
func (m *Mailbox) Code() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.code
}

// LockedEvents returns the published security.account_locked events.
func (m *Mailbox) LockedEvents() []events.AccountLocked {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]events.AccountLocked(nil), m.locked...)
}

// Session wires the login and MFA services over one repository and mailbox.
type Session struct {
	Login   *login.Service
	MFA     *mfa.Service
	Mailbox *Mailbox
}

// New composes the services with one shared lockout policy.
func New(repository *store.Store, signer *token.Service, policy lockout.Config) *Session {
	mailbox := &Mailbox{}
	mfaService := mfa.New(repository, mailbox, nil, time.Now).WithTokenService(signer, time.Hour).WithLockout(policy)
	loginService := login.New(repository, signer, time.Hour, policy).WithMFA(mfaService)
	return &Session{Login: loginService, MFA: mfaService, Mailbox: mailbox}
}

// SignIn performs the password step and completes the challenge with the
// mailed code, returning the issued session.
func (s *Session) SignIn(ctx context.Context, input login.Input) (mfa.Result, error) {
	challenge, err := s.Login.Login(ctx, input)
	if err != nil {
		return mfa.Result{}, fmt.Errorf("password step: %w", err)
	}
	result, err := s.MFA.Verify(ctx, mfa.VerifyInput{Token: challenge.MfaToken, Code: s.Mailbox.Code(), IP: input.IP, UserAgent: input.UserAgent})
	if err != nil {
		return mfa.Result{}, fmt.Errorf("mfa step: %w", err)
	}
	return result, nil
}
