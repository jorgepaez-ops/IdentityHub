// Package mfa implements the mandatory, email-delivered second factor.
package mfa

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/lockout"
	"github.com/jorgepaez/identity-hub/internal/auth/roles"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/events"
)

const (
	ChallengeTTL       = 5 * time.Minute
	MaxAttempts        = 5
	ResendInterval     = time.Minute
	challengeExpiresIn = int(ChallengeTTL / time.Second)
	codeSpace          = 1000000
)

type User struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	Status      string
}

type Challenge struct {
	Token     string
	ExpiresIn int
}

type CreateParams struct {
	UserID       uuid.UUID
	TokenHash    []byte
	CodeHash     []byte
	ExpiresAt    time.Time
	AttemptsLeft int
}

type AuditEvent struct {
	ActorUserID uuid.UUID
	Action      string
	Reason      string
	IP          *netip.Addr
	UserAgent   *string
}

type StoredChallenge struct {
	ID                    uuid.UUID
	User                  User
	CodeHash              []byte
	ExpiresAt, LastSentAt time.Time
	Used                  bool
	AttemptsLeft          int
}

type RefreshToken struct {
	UserID    uuid.UUID
	TokenHash []byte
	FamilyID  uuid.UUID
	IP        *netip.Addr
	UserAgent *string
	ExpiresAt time.Time
}

// VerifyInput carries the request metadata recorded with the resulting
// refresh session. Its IP must come from the trusted API middleware, never
// directly from a forwarding header.
type VerifyInput struct {
	Token     string
	Code      string
	IP        *netip.Addr
	UserAgent *string
}

type Result struct {
	AccessToken, RefreshToken, TokenType string
	ExpiresIn                            int
}

var (
	ErrChallengeInvalid = fmt.Errorf("mfa challenge is unavailable")
	ErrCodeInvalid      = fmt.Errorf("mfa code is invalid")
	ErrResendTooSoon    = fmt.Errorf("mfa resend is rate limited")
)

type Writer interface {
	CreateChallenge(context.Context, CreateParams) error
	GetChallengeForUpdate(context.Context, []byte) (StoredChallenge, error)
	ConsumeChallenge(context.Context, uuid.UUID) error
	RejectChallenge(context.Context, uuid.UUID) (int, error)
	ResendChallenge(context.Context, uuid.UUID, []byte) error
	ListRolesForUser(context.Context, uuid.UUID) ([]string, error)
	CreateRefreshToken(context.Context, RefreshToken) error
	InsertAuditEvent(context.Context, AuditEvent) error
	CountLoginFailuresByAccount(context.Context, uuid.UUID, time.Time) (int64, error)
	LockLoginUser(context.Context, uuid.UUID, time.Time) error
	UpdateLastLogin(context.Context, uuid.UUID) error
}

type Repository interface {
	WithinTransaction(context.Context, func(Writer) error) error
}

type Publisher interface {
	Publish(context.Context, string, any) error
}

type Issuer interface {
	Issue(context.Context, User) (Challenge, error)
}
type Authenticator interface {
	Verify(context.Context, VerifyInput) (Result, error)
	Resend(context.Context, string) error
}

type Service struct {
	repository Repository
	publisher  Publisher
	random     io.Reader
	now        func() time.Time
	tokens     *token.Service
	refreshTTL time.Duration
	lockout    lockout.Config
}

func (s *Service) WithTokenService(tokens *token.Service, refreshTTL time.Duration) *Service {
	s.tokens, s.refreshTTL = tokens, refreshTTL
	return s
}

// WithLockout sets the RF-017 policy applied to rejected codes; composition
// passes the same value the password step uses.
func (s *Service) WithLockout(policy lockout.Config) *Service {
	s.lockout = policy
	return s
}

func New(repository Repository, publisher Publisher, random io.Reader, now func() time.Time) *Service {
	if random == nil {
		random = rand.Reader
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repository: repository, publisher: publisher, random: random, now: now, lockout: lockout.Default()}
}

func (s *Service) Verify(ctx context.Context, input VerifyInput) (Result, error) {
	if s.repository == nil || s.publisher == nil || s.tokens == nil || s.refreshTTL <= 0 || !s.lockout.Valid() {
		return Result{}, fmt.Errorf("mfa service is unavailable")
	}
	raw, err := base64.RawURLEncoding.DecodeString(input.Token)
	if err != nil {
		return Result{}, ErrChallengeInvalid
	}
	if len(input.Code) != 6 {
		return Result{}, ErrCodeInvalid
	}
	for _, c := range input.Code {
		if c < '0' || c > '9' {
			return Result{}, ErrCodeInvalid
		}
	}
	tokenHash := sha256.Sum256(raw)
	codeHash := hashCode(raw, input.Code)
	var result Result
	var verificationErr error
	var lockEvent *events.AccountLocked
	err = s.repository.WithinTransaction(ctx, func(w Writer) error {
		challenge, err := w.GetChallengeForUpdate(ctx, tokenHash[:])
		if err != nil {
			return challengeLookupError(err)
		}
		now := s.now()
		if challenge.Used || !now.Before(challenge.ExpiresAt) || challenge.AttemptsLeft <= 0 {
			return ErrChallengeInvalid
		}
		// A locked or otherwise inactive account no longer verifies open challenges.
		if challenge.User.Status != "active" {
			return ErrChallengeInvalid
		}
		if subtle.ConstantTimeCompare(challenge.CodeHash, codeHash) != 1 {
			lockEvent, err = s.rejectCode(ctx, w, challenge, input, now)
			if err != nil {
				return err
			}
			// Returning the domain error from this callback would roll back the
			// decrement, audit and lock. Commit them, then expose it after the transaction.
			verificationErr = ErrCodeInvalid
			return nil
		}
		if err := w.ConsumeChallenge(ctx, challenge.ID); err != nil {
			return fmt.Errorf("consume mfa challenge: %w", err)
		}
		userRoles, err := w.ListRolesForUser(ctx, challenge.User.ID)
		if err != nil {
			return fmt.Errorf("list mfa user roles: %w", err)
		}
		access, err := s.tokens.Issue(challenge.User.ID.String(), roles.Directory(userRoles))
		if err != nil {
			return fmt.Errorf("issue mfa access token: %w", err)
		}
		refreshRaw := make([]byte, 32)
		if _, err := io.ReadFull(s.random, refreshRaw); err != nil {
			return fmt.Errorf("generate mfa refresh token: %w", err)
		}
		refreshHash := sha256.Sum256(refreshRaw)
		if err := w.CreateRefreshToken(ctx, RefreshToken{UserID: challenge.User.ID, TokenHash: refreshHash[:], FamilyID: uuid.New(), IP: input.IP, UserAgent: input.UserAgent, ExpiresAt: now.Add(s.refreshTTL)}); err != nil {
			return fmt.Errorf("create mfa refresh token: %w", err)
		}
		// The login only succeeds here, once the second factor is proven (D11).
		if err := w.UpdateLastLogin(ctx, challenge.User.ID); err != nil {
			return fmt.Errorf("update last login: %w", err)
		}
		for _, action := range []string{"mfa_code_accepted", "login_succeeded"} {
			if err := w.InsertAuditEvent(ctx, AuditEvent{ActorUserID: challenge.User.ID, Action: action, IP: input.IP, UserAgent: input.UserAgent}); err != nil {
				return fmt.Errorf("audit %s: %w", action, err)
			}
		}
		result = Result{AccessToken: access, RefreshToken: base64.RawURLEncoding.EncodeToString(refreshRaw), TokenType: "Bearer", ExpiresIn: token.AccessTokenExpiresIn}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	if lockEvent != nil {
		if err := s.publisher.Publish(ctx, events.TypeAccountLocked, *lockEvent); err != nil {
			// The lock is already committed and audited; surface the failed
			// notification the way login.Service does.
			return Result{}, errors.Join(verificationErr, fmt.Errorf("publish account locked event: %w", err))
		}
	}
	if verificationErr != nil {
		return Result{}, verificationErr
	}
	return result, nil
}

// rejectCode spends one attempt and records the wrong code as an account
// failure (RF-014), so guessing across successive challenges reaches the same
// lockout as password guessing (RF-017). It returns the event to publish once
// the transaction commits when this rejection locked the account.
func (s *Service) rejectCode(ctx context.Context, w Writer, challenge StoredChallenge, input VerifyInput, now time.Time) (*events.AccountLocked, error) {
	user := challenge.User
	remaining, err := w.RejectChallenge(ctx, challenge.ID)
	if err != nil {
		return nil, fmt.Errorf("reject mfa code: %w", err)
	}
	if err := w.InsertAuditEvent(ctx, AuditEvent{ActorUserID: user.ID, Action: "mfa_code_rejected", IP: input.IP, UserAgent: input.UserAgent}); err != nil {
		return nil, fmt.Errorf("audit mfa rejection: %w", err)
	}
	if remaining == 0 {
		// Informational: the exhausting guess above already counted as a failure.
		if err := w.InsertAuditEvent(ctx, AuditEvent{ActorUserID: user.ID, Action: "mfa_challenge_exhausted", IP: input.IP, UserAgent: input.UserAgent}); err != nil {
			return nil, fmt.Errorf("audit mfa exhaustion: %w", err)
		}
	}
	outcome, err := s.lockout.EvaluateAccount(ctx, w, user.ID, now)
	if err != nil || !outcome.Locked {
		return nil, err
	}
	if err := w.InsertAuditEvent(ctx, AuditEvent{ActorUserID: user.ID, Action: "account_locked", Reason: "mfa_code_rejected", IP: input.IP, UserAgent: input.UserAgent}); err != nil {
		return nil, fmt.Errorf("audit account locked: %w", err)
	}
	event := events.AccountLocked{Envelope: events.NewEnvelope(events.TypeAccountLocked, "")}
	event.Data.UserID, event.Data.Email, event.Data.DisplayName = user.ID, user.Email, user.DisplayName
	event.Data.LockedUntil, event.Data.FailedAttempts = outcome.LockedUntil, outcome.FailedAttempts
	if input.IP != nil {
		event.Data.IP = input.IP.String()
	}
	return &event, nil
}

func (s *Service) Resend(ctx context.Context, encodedToken string) error {
	raw, err := base64.RawURLEncoding.DecodeString(encodedToken)
	if err != nil {
		return ErrChallengeInvalid
	}
	tokenHash := sha256.Sum256(raw)
	return s.repository.WithinTransaction(ctx, func(w Writer) error {
		challenge, err := w.GetChallengeForUpdate(ctx, tokenHash[:])
		if err != nil {
			return challengeLookupError(err)
		}
		if challenge.Used || !s.now().Before(challenge.ExpiresAt) || challenge.User.Status != "active" {
			return ErrChallengeInvalid
		}
		if s.now().Sub(challenge.LastSentAt) < ResendInterval {
			return ErrResendTooSoon
		}
		code, err := generateCode(s.random)
		if err != nil {
			return fmt.Errorf("generate mfa resend code: %w", err)
		}
		if err := w.ResendChallenge(ctx, challenge.ID, hashCode(raw, code)); err != nil {
			return fmt.Errorf("resend mfa challenge: %w", err)
		}
		if err := w.InsertAuditEvent(ctx, AuditEvent{ActorUserID: challenge.User.ID, Action: "mfa_code_resent"}); err != nil {
			return fmt.Errorf("audit mfa resend: %w", err)
		}
		event := events.MfaChallengeIssued{Envelope: events.NewEnvelope(events.TypeMfaChallengeIssued, "")}
		event.Data.UserID, event.Data.Email, event.Data.DisplayName, event.Data.Code, event.Data.ExpiresAt = challenge.User.ID, challenge.User.Email, challenge.User.DisplayName, code, challenge.ExpiresAt
		if err := s.publisher.Publish(ctx, events.TypeMfaChallengeIssued, event); err != nil {
			return fmt.Errorf("publish mfa resend: %w", err)
		}
		return nil
	})
}

// Issue records only hashes, then publishes the raw code solely for worker
// delivery. The public token is URL-safe and likewise never persisted; the code
// hash is keyed with it, so database read access alone cannot enumerate the
// six-digit space.
func (s *Service) Issue(ctx context.Context, user User) (Challenge, error) {
	if s.repository == nil || s.publisher == nil {
		return Challenge{}, fmt.Errorf("mfa service is unavailable")
	}
	rawToken := make([]byte, 32)
	if _, err := io.ReadFull(s.random, rawToken); err != nil {
		return Challenge{}, fmt.Errorf("generate mfa token: %w", err)
	}
	code, err := generateCode(s.random)
	if err != nil {
		return Challenge{}, fmt.Errorf("generate mfa code: %w", err)
	}
	tokenHash, codeHash := sha256.Sum256(rawToken), hashCode(rawToken, code)
	expiresAt := s.now().UTC().Add(ChallengeTTL)
	if err := s.repository.WithinTransaction(ctx, func(w Writer) error {
		if err := w.CreateChallenge(ctx, CreateParams{UserID: user.ID, TokenHash: tokenHash[:], CodeHash: codeHash, ExpiresAt: expiresAt, AttemptsLeft: MaxAttempts}); err != nil {
			return fmt.Errorf("create mfa challenge: %w", err)
		}
		if err := w.InsertAuditEvent(ctx, AuditEvent{ActorUserID: user.ID, Action: "mfa_challenge_issued"}); err != nil {
			return fmt.Errorf("audit mfa challenge issued: %w", err)
		}
		event := events.MfaChallengeIssued{Envelope: events.NewEnvelope(events.TypeMfaChallengeIssued, "")}
		event.Data.UserID, event.Data.Email, event.Data.DisplayName, event.Data.Code, event.Data.ExpiresAt = user.ID, user.Email, user.DisplayName, code, expiresAt
		if err := s.publisher.Publish(ctx, events.TypeMfaChallengeIssued, event); err != nil {
			return fmt.Errorf("publish mfa challenge: %w", err)
		}
		return nil
	}); err != nil {
		return Challenge{}, err
	}
	return Challenge{Token: base64.RawURLEncoding.EncodeToString(rawToken), ExpiresIn: challengeExpiresIn}, nil
}

// challengeLookupError keeps an unknown token (the store reports it as
// ErrChallengeInvalid) apart from storage failures, which must surface as 5xx.
func challengeLookupError(err error) error {
	if errors.Is(err, ErrChallengeInvalid) {
		return ErrChallengeInvalid
	}
	return fmt.Errorf("get mfa challenge: %w", err)
}

// generateCode draws a uniformly distributed six-digit code by rejection
// sampling (crypto/rand.Int), avoiding the modulo bias of reducing a uint32.
func generateCode(random io.Reader) (string, error) {
	n, err := rand.Int(random, big.NewInt(codeSpace))
	if err != nil {
		return "", fmt.Errorf("draw mfa code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// hashCode is HMAC-SHA256 keyed with the raw challenge token, which only the
// client and the verifier hold; the database keeps neither.
func hashCode(rawToken []byte, code string) []byte {
	mac := hmac.New(sha256.New, rawToken)
	mac.Write([]byte(code))
	return mac.Sum(nil)
}
