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
	"log/slog"
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
	ChallengeTTL   = 5 * time.Minute
	MaxAttempts    = 5
	ResendInterval = time.Minute
	// IssuanceWindow and MaxIssuancesPerWindow cap how many challenges (and
	// mails) one account can trigger by logging in repeatedly. The window is
	// its own constant rather than the configurable RF-017 failure window: that
	// one is tuned for wrong credentials, this one bounds successful ones.
	IssuanceWindow        = 15 * time.Minute
	MaxIssuancesPerWindow = 5
	compensationTimeout   = 5 * time.Second
	// StatusActive is the only account status that may verify or resend a code.
	StatusActive       = "active"
	challengeExpiresIn = int(ChallengeTTL / time.Second)
	// codeDigits is the code length; the length check, the code space and the
	// zero-padded format all derive from it.
	codeDigits = 6
)

// codeSpace is 10^codeDigits, the number of distinct codes.
var codeSpace = new(big.Int).Exp(big.NewInt(10), big.NewInt(codeDigits), nil)

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
	ID           uuid.UUID
	UserID       uuid.UUID
	TokenHash    []byte
	CodeHash     []byte
	ExpiresAt    time.Time
	AttemptsLeft int32
	// SentAt is the service-clock time recorded as both created_at and
	// last_sent_at, so the window queries and the resend comparison share it.
	SentAt time.Time
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
	// ErrIssuanceLimited means the account already received
	// MaxIssuancesPerWindow challenges within IssuanceWindow.
	ErrIssuanceLimited = fmt.Errorf("mfa challenge issuance is rate limited")
	// ErrDeliveryUnavailable means a committed challenge or resend could not be
	// delivered after the commit (D16). Undelivered initial challenges are
	// canceled, while a failed resend restores its previous send time.
	ErrDeliveryUnavailable = fmt.Errorf("mfa code delivery is unavailable")
)

type Writer interface {
	// LockChallengeIssuance serializes concurrent issuance for one account so
	// the per-window limit cannot be overshot.
	LockChallengeIssuance(context.Context, uuid.UUID) error
	CountChallengesSince(context.Context, uuid.UUID, time.Time) (int, error)
	// SupersedeOpenChallenges marks the account's open challenges as used.
	SupersedeOpenChallenges(context.Context, uuid.UUID, time.Time) error
	CreateChallenge(context.Context, CreateParams) error
	DeleteChallenge(context.Context, uuid.UUID) error
	RestoreResend(ctx context.Context, challengeID uuid.UUID, expectedHash []byte, previousHash []byte, sentAt time.Time, previousSentAt time.Time) error
	GetChallengeForUpdate(context.Context, []byte) (StoredChallenge, error)
	ConsumeChallenge(context.Context, uuid.UUID) error
	RejectChallenge(context.Context, uuid.UUID) (int, error)
	// ResendChallenge stores the new code hash; sentAt comes from the service
	// clock, the same one the resend window is measured with.
	ResendChallenge(context.Context, uuid.UUID, []byte, time.Time) error
	ListRolesForUser(context.Context, uuid.UUID) ([]string, error)
	CreateRefreshToken(context.Context, RefreshToken) error
	InsertAuditEvent(context.Context, AuditEvent) error
	CountLoginFailuresByAccount(context.Context, uuid.UUID, time.Time) (int64, error)
	LockLoginUser(context.Context, uuid.UUID, time.Time) error
	UpdateLastLogin(context.Context, uuid.UUID) error
}

type Repository interface {
	WithinMFATransaction(context.Context, func(Writer) error) error
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
	logger     *slog.Logger
}

// WithLogger sets where publication failures that must not change the client
// response are reported. Nil disables logging.
func (s *Service) WithLogger(logger *slog.Logger) *Service {
	s.logger = logger
	return s
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
	if len(input.Code) != codeDigits {
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
	err = s.repository.WithinMFATransaction(ctx, func(w Writer) error {
		challenge, err := w.GetChallengeForUpdate(ctx, tokenHash[:])
		if err != nil {
			return challengeLookupError(err)
		}
		now := s.now()
		if challenge.Used || !now.Before(challenge.ExpiresAt) || challenge.AttemptsLeft <= 0 {
			return ErrChallengeInvalid
		}
		// A locked or otherwise inactive account no longer verifies open challenges.
		if challenge.User.Status != StatusActive {
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
		familyID := uuid.New()
		access, err := s.tokens.IssueForSession(challenge.User.ID.String(), roles.Directory(userRoles), familyID.String())
		if err != nil {
			return fmt.Errorf("issue mfa access token: %w", err)
		}
		refreshRaw := make([]byte, 32)
		if _, err := io.ReadFull(s.random, refreshRaw); err != nil {
			return fmt.Errorf("generate mfa refresh token: %w", err)
		}
		refreshHash := sha256.Sum256(refreshRaw)
		if err := w.CreateRefreshToken(ctx, RefreshToken{UserID: challenge.User.ID, TokenHash: refreshHash[:], FamilyID: familyID, IP: input.IP, UserAgent: input.UserAgent, ExpiresAt: now.Add(s.refreshTTL)}); err != nil {
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
			// The lock is already committed and audited; the client still gets
			// the 401, so the failed notification is logged here (same shape as
			// the login publisher) and joined the way login.Service does.
			if s.logger != nil {
				s.logger.Warn("security event could not be published", "event_type", events.TypeAccountLocked, "user_id", lockEvent.Data.UserID, "error", err)
			}
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
	var event events.MfaChallengeIssued
	var challengeID uuid.UUID
	var newHash, previousHash []byte
	var sentAt, previousSentAt time.Time
	if err := s.repository.WithinMFATransaction(ctx, func(w Writer) error {
		challenge, err := w.GetChallengeForUpdate(ctx, tokenHash[:])
		if err != nil {
			return challengeLookupError(err)
		}
		// One clock reading serves the expiry check, the window comparison and
		// the stored last_sent_at.
		now := s.now()
		if challenge.Used || !now.Before(challenge.ExpiresAt) || challenge.User.Status != StatusActive {
			return ErrChallengeInvalid
		}
		if now.Sub(challenge.LastSentAt) < ResendInterval {
			return ErrResendTooSoon
		}
		code, err := generateCode(s.random)
		if err != nil {
			return fmt.Errorf("generate mfa resend code: %w", err)
		}
		challengeID, sentAt = challenge.ID, now
		newHash, previousHash = hashCode(raw, code), append([]byte(nil), challenge.CodeHash...)
		previousSentAt = challenge.LastSentAt
		if err := w.ResendChallenge(ctx, challengeID, newHash, sentAt); err != nil {
			return fmt.Errorf("resend mfa challenge: %w", err)
		}
		if err := w.InsertAuditEvent(ctx, AuditEvent{ActorUserID: challenge.User.ID, Action: "mfa_code_resent"}); err != nil {
			return fmt.Errorf("audit mfa resend: %w", err)
		}
		event = challengeEvent(challenge.User, code, challenge.ExpiresAt)
		return nil
	}); err != nil {
		return err
	}
	if err := s.publishCode(ctx, event); err != nil {
		compensationCtx, cancel := compensationContext(ctx)
		defer cancel()
		if restoreErr := s.repository.WithinMFATransaction(compensationCtx, func(w Writer) error {
			if err := w.RestoreResend(compensationCtx, challengeID, newHash, previousHash, sentAt, previousSentAt); err != nil {
				return fmt.Errorf("restore failed mfa resend: %w", err)
			}
			return nil
		}); restoreErr != nil {
			s.logCompensationFailure("restore_mfa_resend", restoreErr)
			// Do not keep ErrDeliveryUnavailable in this path: the compensating
			// database failure must surface as infrastructure failure (5xx).
			return fmt.Errorf("restore undelivered mfa resend: %w", restoreErr)
		}
		return err
	}
	return nil
}

func compensationContext(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(parent), compensationTimeout)
}

func (s *Service) logCompensationFailure(operation string, err error) {
	if s.logger != nil {
		s.logger.Error("mfa delivery compensation failed", "operation", operation, "error", err)
	}
}

// publishCode sends the code once the transaction that stored it has
// committed (D16): a broker outage or a rolled-back commit can then neither
// hold a database connection nor mail a code for a challenge that does not
// exist. On failure Issue cancels its challenge and Resend restores its prior
// code timestamp, allowing an immediate retry.
func (s *Service) publishCode(ctx context.Context, event events.MfaChallengeIssued) error {
	if err := s.publisher.Publish(ctx, events.TypeMfaChallengeIssued, event); err != nil {
		return fmt.Errorf("%w: publish mfa code: %w", ErrDeliveryUnavailable, err)
	}
	return nil
}

func challengeEvent(user User, code string, expiresAt time.Time) events.MfaChallengeIssued {
	event := events.MfaChallengeIssued{Envelope: events.NewEnvelope(events.TypeMfaChallengeIssued, "")}
	event.Data.UserID, event.Data.Email, event.Data.DisplayName, event.Data.Code, event.Data.ExpiresAt = user.ID, user.Email, user.DisplayName, code, expiresAt
	return event
}

// Issue records only hashes, then publishes the raw code solely for worker
// delivery once the transaction has committed (D16). The public token is
// URL-safe and likewise never persisted; the code hash is keyed with it, so
// database read access alone cannot enumerate the six-digit space. A new
// challenge supersedes the account's open ones, and at most
// MaxIssuancesPerWindow may be issued per IssuanceWindow.
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
	challengeID := uuid.New()
	now := s.now().UTC()
	expiresAt := now.Add(ChallengeTTL)
	if err := s.repository.WithinMFATransaction(ctx, func(w Writer) error {
		if err := w.LockChallengeIssuance(ctx, user.ID); err != nil {
			return fmt.Errorf("lock mfa issuance: %w", err)
		}
		issued, err := w.CountChallengesSince(ctx, user.ID, now.Add(-IssuanceWindow))
		if err != nil {
			return fmt.Errorf("count mfa challenges: %w", err)
		}
		if issued >= MaxIssuancesPerWindow {
			return ErrIssuanceLimited
		}
		if err := w.SupersedeOpenChallenges(ctx, user.ID, now); err != nil {
			return fmt.Errorf("supersede mfa challenges: %w", err)
		}
		if err := w.CreateChallenge(ctx, CreateParams{ID: challengeID, UserID: user.ID, TokenHash: tokenHash[:], CodeHash: codeHash, ExpiresAt: expiresAt, AttemptsLeft: MaxAttempts, SentAt: now}); err != nil {
			return fmt.Errorf("create mfa challenge: %w", err)
		}
		if err := w.InsertAuditEvent(ctx, AuditEvent{ActorUserID: user.ID, Action: "mfa_challenge_issued"}); err != nil {
			return fmt.Errorf("audit mfa challenge issued: %w", err)
		}
		return nil
	}); err != nil {
		return Challenge{}, err
	}
	if err := s.publishCode(ctx, challengeEvent(user, code, expiresAt)); err != nil {
		compensationCtx, cancel := compensationContext(ctx)
		defer cancel()
		if cancelErr := s.repository.WithinMFATransaction(compensationCtx, func(w Writer) error {
			if err := w.DeleteChallenge(compensationCtx, challengeID); err != nil {
				return fmt.Errorf("cancel undelivered mfa challenge: %w", err)
			}
			return nil
		}); cancelErr != nil {
			s.logCompensationFailure("delete_mfa_challenge", cancelErr)
			// A failed compensation is an infrastructure fault, not a delivery
			// outcome; avoid mapping it to the retryable MFA domain error.
			return Challenge{}, fmt.Errorf("cancel undelivered mfa challenge: %w", cancelErr)
		}
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
	n, err := rand.Int(random, codeSpace)
	if err != nil {
		return "", fmt.Errorf("draw mfa code: %w", err)
	}
	return fmt.Sprintf("%0*d", codeDigits, n.Int64()), nil
}

// hashCode is HMAC-SHA256 keyed with the raw challenge token, which only the
// client and the verifier hold; the database keeps neither.
func hashCode(rawToken []byte, code string) []byte {
	mac := hmac.New(sha256.New, rawToken)
	mac.Write([]byte(code))
	return mac.Sum(nil)
}
