// Package login implements password authentication and refresh-session issuance.
package login

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrMFAUnavailable     = errors.New("multi-factor authentication is not supported")
	ErrAccountLocked      = errors.New("account is locked")
	ErrIPRateLimited      = errors.New("login IP is rate limited")
)

const accessTokenExpiresIn = 900

const (
	defaultAccountMaxFailures = 5
	defaultIPMaxFailures      = 20
	defaultFailureWindow      = 15 * time.Minute
	defaultLockoutDuration    = 15 * time.Minute
)

type Status string

const (
	StatusActive Status = "active"
	StatusLocked Status = "locked"
)

type User struct {
	ID           uuid.UUID
	Email        string
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

type Result struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int
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
	Type   string
	UserID uuid.UUID
	IP     *netip.Addr
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
	UpdateLoginSuccess(context.Context, uuid.UUID, string) error
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
	now        func() time.Time
}

// WithEventPublisher wires the broker publisher used to notify
// security.account_locked, mirroring refresh.Service.WithEventPublisher.
func (s *Service) WithEventPublisher(publisher EventPublisher) *Service {
	s.publisher = publisher
	return s
}

// LockoutConfig controls the independent account and IP sliding-window limits.
type LockoutConfig struct {
	AccountMaxFailures int
	IPMaxFailures      int
	FailureWindow      time.Duration
	LockoutDuration    time.Duration
}

func defaultLockoutConfig() LockoutConfig {
	return LockoutConfig{
		AccountMaxFailures: defaultAccountMaxFailures,
		IPMaxFailures:      defaultIPMaxFailures,
		FailureWindow:      defaultFailureWindow,
		LockoutDuration:    defaultLockoutDuration,
	}
}

// New creates the login service. The optional lockout configuration retains
// compatibility with existing callers while allowing composition to inject the
// environment-derived policy.
func New(repository Repository, tokens *token.Service, refreshTTL time.Duration, lockout ...LockoutConfig) *Service {
	policy := defaultLockoutConfig()
	if len(lockout) == 1 {
		policy = lockout[0]
	}
	return &Service{repository: repository, tokens: tokens, refreshTTL: refreshTTL, lockout: policy, now: time.Now}
}

func (s *Service) Login(ctx context.Context, input Input) (Result, error) {
	if s.repository == nil || s.tokens == nil || s.refreshTTL <= 0 || !s.lockout.valid() {
		return Result{}, fmt.Errorf("login service is unavailable")
	}
	var result Result
	var authenticationErr error
	var lockEvent *SecurityEvent
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
			locked, err := s.recordAccountFailure(ctx, writer, user, input, "invalid_credentials", now)
			if err != nil {
				return err
			}
			if locked {
				lockEvent = &SecurityEvent{Type: "security.account_locked", UserID: user.ID, IP: input.IP}
			}
			authenticationErr = ErrInvalidCredentials
			return nil
		}
		if user.MFAEnabled {
			locked, err := s.recordAccountFailure(ctx, writer, user, input, "mfa_not_supported", now)
			if err != nil {
				return err
			}
			if locked {
				lockEvent = &SecurityEvent{Type: "security.account_locked", UserID: user.ID, IP: input.IP}
			}
			authenticationErr = ErrMFAUnavailable
			return nil
		}

		updatedHash := user.PasswordHash
		if password.NeedsRehash(user.PasswordHash) {
			updatedHash, err = password.Hash(input.Password)
			if err != nil {
				return fmt.Errorf("rehash password: %w", err)
			}
		}
		roles, err := writer.ListRolesForUser(ctx, user.ID)
		if err != nil {
			return fmt.Errorf("list user roles: %w", err)
		}
		accessToken, err := s.tokens.Issue(user.ID.String(), roles)
		if err != nil {
			return fmt.Errorf("issue access token: %w", err)
		}
		refreshRaw := make([]byte, 32)
		if _, err := rand.Read(refreshRaw); err != nil {
			return fmt.Errorf("generate refresh token: %w", err)
		}
		refreshHash := sha256.Sum256(refreshRaw)
		if err := writer.UpdateLoginSuccess(ctx, user.ID, updatedHash); err != nil {
			return fmt.Errorf("update login success: %w", err)
		}
		if err := writer.CreateRefreshToken(ctx, RefreshToken{UserID: user.ID, TokenHash: refreshHash[:], FamilyID: uuid.New(), IP: input.IP, UserAgent: input.UserAgent, ExpiresAt: s.now().Add(s.refreshTTL)}); err != nil {
			return fmt.Errorf("create refresh token: %w", err)
		}
		if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: &user.ID, Action: "login_succeeded", IP: input.IP, UserAgent: input.UserAgent}); err != nil {
			return fmt.Errorf("record successful login audit: %w", err)
		}
		result = Result{AccessToken: accessToken, RefreshToken: string(refreshRaw), TokenType: "Bearer", ExpiresIn: accessTokenExpiresIn}
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
	return result, nil
}

// recordAccountFailure records the failed attempt and, once it reaches the
// threshold, locks the account and audits account_locked in the same
// transaction. It reports whether the account was just locked, so the caller
// can publish security.account_locked after the transaction commits.
func (s *Service) recordAccountFailure(ctx context.Context, writer Writer, user User, input Input, reason string, now time.Time) (bool, error) {
	if err := s.recordFailure(ctx, writer, &user.ID, input, reason); err != nil {
		return false, err
	}
	if user.Status != StatusActive {
		return false, nil
	}
	failures, err := writer.CountLoginFailuresByAccount(ctx, user.ID, now.Add(-s.lockout.FailureWindow))
	if err != nil {
		return false, fmt.Errorf("count login failures by account: %w", err)
	}
	if failures < int64(s.lockout.AccountMaxFailures) {
		return false, nil
	}
	if err := writer.LockLoginUser(ctx, user.ID, now.Add(s.lockout.LockoutDuration)); err != nil {
		return false, fmt.Errorf("lock login user: %w", err)
	}
	if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: &user.ID, Action: "account_locked", Reason: reason, IP: input.IP, UserAgent: input.UserAgent}); err != nil {
		return false, fmt.Errorf("record account locked audit: %w", err)
	}
	return true, nil
}

func (c LockoutConfig) valid() bool {
	return c.AccountMaxFailures > 0 && c.IPMaxFailures > 0 && c.FailureWindow > 0 && c.LockoutDuration > 0
}

func (s *Service) recordFailure(ctx context.Context, writer Writer, actorID *uuid.UUID, input Input, reason string) error {
	if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: actorID, Action: "login_failed", Reason: reason, IP: input.IP, UserAgent: input.UserAgent}); err != nil {
		return fmt.Errorf("record failed login audit: %w", err)
	}
	return nil
}
