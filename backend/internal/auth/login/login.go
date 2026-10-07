// Package login implements password authentication and refresh-session issuance.
package login

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/lockout"
	"github.com/jorgepaez/identity-hub/internal/auth/mfa"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account is locked")
	ErrIPRateLimited      = errors.New("login IP is rate limited")
)

type Status string

const (
	StatusActive Status = "active"
	StatusLocked Status = "locked"
)

type User struct {
	ID           uuid.UUID
	Email        string
	DisplayName  string
	PasswordHash string
	Status       Status
	LockedUntil  *time.Time
}

type Input struct {
	Email     string
	Password  string
	IP        *netip.Addr
	UserAgent *string
}

// Result is always an MFA challenge: no session exists until the code is
// verified (D11).
type Result struct {
	MfaToken  string
	ExpiresIn int
}

type AuditEvent struct {
	ActorUserID *uuid.UUID
	Action      string
	Reason      string
	IP          *netip.Addr
	UserAgent   *string
}

// SecurityEvent is published to the broker for security-relevant occurrences
// that also merit a notification, mirroring refresh.SecurityEvent.
type SecurityEvent struct {
	Type           string
	UserID         uuid.UUID
	IP             *netip.Addr
	LockedUntil    time.Time
	FailedAttempts int
}

type EventPublisher interface {
	PublishSecurityEvent(context.Context, SecurityEvent) error
}

type Writer interface {
	GetLoginUserByEmail(context.Context, string) (User, error)
	CountLoginFailuresByAccount(context.Context, uuid.UUID, time.Time) (int64, error)
	CountLoginFailuresByIP(context.Context, netip.Addr, time.Time) (int64, error)
	LockLoginUser(context.Context, uuid.UUID, time.Time) error
	UnlockLoginUser(context.Context, uuid.UUID) error
	UpdatePasswordHash(context.Context, uuid.UUID, string) error
	InsertAuditEvent(context.Context, AuditEvent) error
}

type Repository interface {
	WithinLoginTransaction(context.Context, func(Writer) error) error
}

type Authenticator interface {
	Login(context.Context, Input) (Result, error)
}

type Service struct {
	repository Repository
	lockout    LockoutConfig
	publisher  EventPublisher
	mfa        mfa.Issuer
	now        func() time.Time
}

// WithMFA makes email MFA mandatory after password verification (D11).
func (s *Service) WithMFA(issuer mfa.Issuer) *Service { s.mfa = issuer; return s }

// WithEventPublisher wires the broker publisher used to notify
// security.account_locked, mirroring refresh.Service.WithEventPublisher.
func (s *Service) WithEventPublisher(publisher EventPublisher) *Service {
	s.publisher = publisher
	return s
}

// LockoutConfig controls the independent account and IP sliding-window limits.
// It is shared with the MFA verification step so both enforce one policy.
type LockoutConfig = lockout.Config

// New creates the login service. The optional lockout configuration retains
// compatibility with existing callers while allowing composition to inject the
// environment-derived policy. Sessions are issued by the mfa package, so this
// service needs no token signer or refresh lifetime.
func New(repository Repository, policies ...LockoutConfig) *Service {
	policy := lockout.Default()
	if len(policies) == 1 {
		policy = policies[0]
	}
	return &Service{repository: repository, lockout: policy, now: time.Now}
}

// loginOutcome carries what the login transaction decided: the authentication
// error to report (nil on success), the account-locked event to publish after
// commit, and the user to issue the MFA challenge for.
type loginOutcome struct {
	authenticationErr error
	lockEvent         *SecurityEvent
	mfaUser           mfa.User
}

func (s *Service) Login(ctx context.Context, input Input) (Result, error) {
	if s.repository == nil || s.mfa == nil || !s.lockout.Valid() {
		return Result{}, fmt.Errorf("login service is unavailable")
	}
	var outcome loginOutcome
	err := s.repository.WithinLoginTransaction(ctx, func(writer Writer) error {
		return s.authenticate(ctx, writer, input, s.now(), &outcome)
	})
	if err != nil {
		return Result{}, err
	}
	if err := s.publishLockEvent(ctx, outcome); err != nil {
		return Result{}, err
	}
	if outcome.authenticationErr != nil {
		return Result{}, outcome.authenticationErr
	}
	challenge, err := s.mfa.Issue(ctx, outcome.mfaUser)
	if err != nil {
		return Result{}, fmt.Errorf("issue mfa challenge: %w", err)
	}
	return Result{MfaToken: challenge.Token, ExpiresIn: challenge.ExpiresIn}, nil
}

// authenticate runs every step of the password check inside the login
// transaction and records the decision in outcome. A nil return with
// outcome.authenticationErr set means a rejection that must still commit.
func (s *Service) authenticate(ctx context.Context, writer Writer, input Input, now time.Time, outcome *loginOutcome) error {
	limited, err := s.ipRateLimited(ctx, writer, input, now)
	if err != nil {
		return err
	}
	if limited {
		outcome.authenticationErr = ErrIPRateLimited
		return nil
	}

	user, err := writer.GetLoginUserByEmail(ctx, input.Email)
	if err != nil {
		return s.rejectUnknownUser(ctx, writer, input, err, outcome)
	}
	locked, err := s.resolveLock(ctx, writer, &user, now)
	if err != nil {
		return err
	}
	if locked {
		outcome.authenticationErr = ErrAccountLocked
		return nil
	}

	valid, err := password.Verify(input.Password, user.PasswordHash)
	if err := tolerateInvalidPassword(err); err != nil {
		return fmt.Errorf("verify password: %w", err)
	}
	if !valid || user.Status != StatusActive {
		return s.rejectKnownUser(ctx, writer, user, input, now, outcome)
	}
	if err := s.rehashIfNeeded(ctx, writer, user, input.Password); err != nil {
		return err
	}
	outcome.mfaUser = mfa.User{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName}
	return nil
}

// tolerateInvalidPassword drops password.InvalidPasswordError (a verification
// verdict, not a failure) and returns any other error unchanged.
func tolerateInvalidPassword(err error) error {
	var invalidPasswordErr *password.InvalidPasswordError
	if err != nil && !errors.As(err, &invalidPasswordErr) {
		return err
	}
	return nil
}

// ipRateLimited reports whether the source IP reached its failure limit within
// the sliding window. Requests without an IP are never IP-limited.
func (s *Service) ipRateLimited(ctx context.Context, writer Writer, input Input, now time.Time) (bool, error) {
	if input.IP == nil {
		return false, nil
	}
	failures, err := writer.CountLoginFailuresByIP(ctx, *input.IP, now.Add(-s.lockout.FailureWindow))
	if err != nil {
		return false, fmt.Errorf("count login failures by IP: %w", err)
	}
	return failures >= int64(s.lockout.IPMaxFailures), nil
}

// rejectUnknownUser handles a failed user lookup. For an unknown email it
// verifies a decoy password (equalizing timing against account enumeration),
// audits the failure and reports invalid credentials; any other lookup error
// aborts the transaction.
func (s *Service) rejectUnknownUser(ctx context.Context, writer Writer, input Input, lookupErr error, outcome *loginOutcome) error {
	if !errors.Is(lookupErr, pgx.ErrNoRows) {
		return fmt.Errorf("get login user: %w", lookupErr)
	}
	if err := tolerateInvalidPassword(password.VerifyDecoy(input.Password)); err != nil {
		return fmt.Errorf("verify decoy password: %w", err)
	}
	if err := s.recordFailure(ctx, writer, nil, input, "invalid_credentials"); err != nil {
		return err
	}
	outcome.authenticationErr = ErrInvalidCredentials
	return nil
}

// resolveLock reports whether the account is still locked (permanently when
// LockedUntil is nil). An expired lock is released and user is updated to the
// active state so the password check can proceed.
func (s *Service) resolveLock(ctx context.Context, writer Writer, user *User, now time.Time) (bool, error) {
	if user.Status != StatusLocked {
		return false, nil
	}
	if user.LockedUntil == nil || now.Before(*user.LockedUntil) {
		return true, nil
	}
	if err := writer.UnlockLoginUser(ctx, user.ID); err != nil {
		return false, fmt.Errorf("unlock expired login lock: %w", err)
	}
	user.Status = StatusActive
	user.LockedUntil = nil
	return false, nil
}

// rejectKnownUser records the failed attempt for an existing account and, when
// it just locked the account, stages the security.account_locked event to be
// published after the transaction commits.
func (s *Service) rejectKnownUser(ctx context.Context, writer Writer, user User, input Input, now time.Time, outcome *loginOutcome) error {
	lock, err := s.recordAccountFailure(ctx, writer, user, input, "invalid_credentials", now)
	if err != nil {
		return err
	}
	if lock.Locked {
		outcome.lockEvent = &SecurityEvent{Type: "security.account_locked", UserID: user.ID, IP: input.IP, LockedUntil: lock.LockedUntil, FailedAttempts: lock.FailedAttempts}
	}
	outcome.authenticationErr = ErrInvalidCredentials
	return nil
}

// rehashIfNeeded transparently upgrades an outdated password hash. Success
// bookkeeping (audit, last_login_at) waits for the MFA code (D11); only the
// rehash belongs to the password step.
func (s *Service) rehashIfNeeded(ctx context.Context, writer Writer, user User, plaintext string) error {
	if !password.NeedsRehash(user.PasswordHash) {
		return nil
	}
	updatedHash, err := password.Hash(plaintext)
	if err != nil {
		return fmt.Errorf("rehash password: %w", err)
	}
	if err := writer.UpdatePasswordHash(ctx, user.ID, updatedHash); err != nil {
		return fmt.Errorf("update password hash: %w", err)
	}
	return nil
}

// publishLockEvent notifies the broker once the transaction has committed. A
// publish failure is joined with the authentication error instead of dropped.
func (s *Service) publishLockEvent(ctx context.Context, outcome loginOutcome) error {
	if outcome.lockEvent == nil || s.publisher == nil {
		return nil
	}
	if err := s.publisher.PublishSecurityEvent(ctx, *outcome.lockEvent); err != nil {
		// The lock is already committed and audited inside the transaction
		// above; a notification failure must not be silently dropped, so it
		// surfaces the same way refresh.Service does for reuse detection.
		return errors.Join(outcome.authenticationErr, fmt.Errorf("publish account locked event: %w", err))
	}
	return nil
}

// recordAccountFailure records the failed attempt and, once it reaches the
// threshold, locks the account and audits account_locked in the same
// transaction. It reports whether the account was just locked, so the caller
// can publish security.account_locked after the transaction commits.
func (s *Service) recordAccountFailure(ctx context.Context, writer Writer, user User, input Input, reason string, now time.Time) (lockout.Outcome, error) {
	if err := s.recordFailure(ctx, writer, &user.ID, input, reason); err != nil {
		return lockout.Outcome{}, err
	}
	if user.Status != StatusActive {
		return lockout.Outcome{}, nil
	}
	outcome, err := s.lockout.EvaluateAccount(ctx, writer, user.ID, now)
	if err != nil || !outcome.Locked {
		return outcome, err
	}
	if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: &user.ID, Action: "account_locked", Reason: reason, IP: input.IP, UserAgent: input.UserAgent}); err != nil {
		return lockout.Outcome{}, fmt.Errorf("record account locked audit: %w", err)
	}
	return outcome, nil
}

func (s *Service) recordFailure(ctx context.Context, writer Writer, actorID *uuid.UUID, input Input, reason string) error {
	if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: actorID, Action: "login_failed", Reason: reason, IP: input.IP, UserAgent: input.UserAgent}); err != nil {
		return fmt.Errorf("record failed login audit: %w", err)
	}
	return nil
}
