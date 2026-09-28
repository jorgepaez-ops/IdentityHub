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
	"github.com/jorgepaez/identity-hub/internal/auth/token"
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
	MFAEnabled   bool
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

type RefreshToken struct {
	UserID    uuid.UUID
	TokenHash []byte
	FamilyID  uuid.UUID
	IP        *netip.Addr
	UserAgent *string
	ExpiresAt time.Time
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
	ListRolesForUser(context.Context, uuid.UUID) ([]string, error)
	UpdatePasswordHash(context.Context, uuid.UUID, string) error
	CreateRefreshToken(context.Context, RefreshToken) error
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
	tokens     *token.Service
	refreshTTL time.Duration
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
// environment-derived policy.
func New(repository Repository, tokens *token.Service, refreshTTL time.Duration, policies ...LockoutConfig) *Service {
	policy := lockout.Default()
	if len(policies) == 1 {
		policy = policies[0]
	}
	return &Service{repository: repository, tokens: tokens, refreshTTL: refreshTTL, lockout: policy, now: time.Now}
}

func (s *Service) Login(ctx context.Context, input Input) (Result, error) {
	if s.repository == nil || s.tokens == nil || s.mfa == nil || s.refreshTTL <= 0 || !s.lockout.Valid() {
		return Result{}, fmt.Errorf("login service is unavailable")
	}
	var authenticationErr error
	var lockEvent *SecurityEvent
	var mfaUser mfa.User
	err := s.repository.WithinLoginTransaction(ctx, func(writer Writer) error {
		now := s.now()
		if input.IP != nil {
			failures, err := writer.CountLoginFailuresByIP(ctx, *input.IP, now.Add(-s.lockout.FailureWindow))
			if err != nil {
				return fmt.Errorf("count login failures by IP: %w", err)
			}
			if failures >= int64(s.lockout.IPMaxFailures) {
				authenticationErr = ErrIPRateLimited
				return nil
			}
		}

		user, err := writer.GetLoginUserByEmail(ctx, input.Email)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				if decoyErr := password.VerifyDecoy(input.Password); decoyErr != nil {
					return fmt.Errorf("verify decoy password: %w", decoyErr)
				}
				if err := s.recordFailure(ctx, writer, nil, input, "invalid_credentials"); err != nil {
					return err
				}
				authenticationErr = ErrInvalidCredentials
				return nil
			}
			return fmt.Errorf("get login user: %w", err)
		}
		if user.Status == StatusLocked {
			if user.LockedUntil == nil || now.Before(*user.LockedUntil) {
				authenticationErr = ErrAccountLocked
				return nil
			}
			if err := writer.UnlockLoginUser(ctx, user.ID); err != nil {
				return fmt.Errorf("unlock expired login lock: %w", err)
			}
			user.Status = StatusActive
			user.LockedUntil = nil
		}

		valid, err := password.Verify(input.Password, user.PasswordHash)
		if err != nil {
			return fmt.Errorf("verify password: %w", err)
		}
		if !valid || user.Status != StatusActive {
			lock, err := s.recordAccountFailure(ctx, writer, user, input, "invalid_credentials", now)
			if err != nil {
				return err
			}
			if lock.Locked {
				lockEvent = &SecurityEvent{Type: "security.account_locked", UserID: user.ID, IP: input.IP, LockedUntil: lock.LockedUntil, FailedAttempts: lock.FailedAttempts}
			}
			authenticationErr = ErrInvalidCredentials
			return nil
		}
		// Success bookkeeping (audit, last_login_at) waits for the MFA code (D11);
		// only the transparent rehash belongs to the password step.
		if password.NeedsRehash(user.PasswordHash) {
			updatedHash, err := password.Hash(input.Password)
			if err != nil {
				return fmt.Errorf("rehash password: %w", err)
			}
			if err := writer.UpdatePasswordHash(ctx, user.ID, updatedHash); err != nil {
				return fmt.Errorf("update password hash: %w", err)
			}
		}
		mfaUser = mfa.User{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	if lockEvent != nil && s.publisher != nil {
		if err := s.publisher.PublishSecurityEvent(ctx, *lockEvent); err != nil {
			// The lock is already committed and audited inside the transaction
			// above; a notification failure must not be silently dropped, so it
			// surfaces the same way refresh.Service does for reuse detection.
			return Result{}, errors.Join(authenticationErr, fmt.Errorf("publish account locked event: %w", err))
		}
	}
	if authenticationErr != nil {
		return Result{}, authenticationErr
	}
	challenge, err := s.mfa.Issue(ctx, mfaUser)
	if err != nil {
		return Result{}, fmt.Errorf("issue mfa challenge: %w", err)
	}
	return Result{MfaToken: challenge.Token, ExpiresIn: challenge.ExpiresIn}, nil
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
