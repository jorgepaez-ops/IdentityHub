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
	// compensationTimeout bounds cleanup after a failed broker publish: five
	// seconds permits a brief database delay without letting request cleanup
	// retain resources indefinitely.
	compensationTimeout = 5 * time.Second
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
type HubSession struct {
	UserID    uuid.UUID
	TokenHash []byte
	FamilyID  uuid.UUID
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
	AccessToken, RefreshToken, HubSessionToken, TokenType string
	ExpiresIn                                             int
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
	CreateHubSession(context.Context, HubSession) error
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
	repository    Repository
	publisher     Publisher
	random        io.Reader
	now           func() time.Time
	tokens        *token.Service
	refreshTTL    time.Duration
	hubSessionTTL time.Duration
	lockout       lockout.Config
	logger        *slog.Logger
}

// WithLogger sets where publication failures that must not change the client
// response are reported. Nil disables logging.
func (s *Service) WithLogger(logger *slog.Logger) *Service {
	s.logger = logger
	return s
}

// WithHubSessionTTL enables the independent SameSite=Lax Hub SSO session.
func (s *Service) WithHubSessionTTL(ttl time.Duration) *Service { s.hubSessionTTL = ttl; return s }

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

// verifyOutcome is what the verification transaction leaves behind for the
// code after commit: the session on success, the domain error to report even
// though the transaction committed (wrong code), and the account-locked event
// to publish once the lock is durable.
type verifyOutcome struct {
	result          Result
	verificationErr error
	lockEvent       *events.AccountLocked
}

// decodeVerifyInput validates the request before any database work: the token
// must decode and the code must be exactly codeDigits decimal digits. It
// returns the raw token bytes.
func decodeVerifyInput(input VerifyInput) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(input.Token)
	if err != nil {
		return nil, ErrChallengeInvalid
	}
	if len(input.Code) != codeDigits {
		return nil, ErrCodeInvalid
	}
	for _, c := range input.Code {
		if c < '0' || c > '9' {
			return nil, ErrCodeInvalid
		}
	}
	return raw, nil
}

func (s *Service) Verify(ctx context.Context, input VerifyInput) (Result, error) {
	if s.repository == nil || s.publisher == nil || s.tokens == nil || s.refreshTTL <= 0 || !s.lockout.Valid() {
		return Result{}, fmt.Errorf("mfa service is unavailable")
	}
	raw, err := decodeVerifyInput(input)
	if err != nil {
		return Result{}, err
	}
	tokenHash := sha256.Sum256(raw)
	codeHash := hashCode(raw, input.Code)
	var outcome verifyOutcome
	err = s.repository.WithinMFATransaction(ctx, func(w Writer) error {
		return s.verifyChallenge(ctx, w, tokenHash[:], codeHash, input, &outcome)
	})
	if err != nil {
		return Result{}, err
	}
	return s.finishVerify(ctx, outcome)
}

// verifyChallenge runs inside the verification transaction: it loads and
// checks the challenge, then either rejects the code or issues the session,
// recording the result in outcome.
func (s *Service) verifyChallenge(ctx context.Context, w Writer, tokenHash, codeHash []byte, input VerifyInput, outcome *verifyOutcome) error {
	challenge, err := w.GetChallengeForUpdate(ctx, tokenHash)
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
		outcome.lockEvent, err = s.rejectCode(ctx, w, challenge, input, now)
		if err != nil {
			return err
		}
		// Returning the domain error from this callback would roll back the
		// decrement, audit and lock. Commit them, then expose it after the transaction.
		outcome.verificationErr = ErrCodeInvalid
		return nil
	}
	outcome.result, err = s.issueSession(ctx, w, challenge, input, now)
	return err
}

// issueSession consumes the proven challenge and creates the session: access
// and refresh tokens, the optional hub session, the last-login stamp and the
// audit trail. Random bytes are read refresh first, then hub.
func (s *Service) issueSession(ctx context.Context, w Writer, challenge StoredChallenge, input VerifyInput, now time.Time) (Result, error) {
	if err := w.ConsumeChallenge(ctx, challenge.ID); err != nil {
		return Result{}, fmt.Errorf("consume mfa challenge: %w", err)
	}
	userRoles, err := w.ListRolesForUser(ctx, challenge.User.ID)
	if err != nil {
		return Result{}, fmt.Errorf("list mfa user roles: %w", err)
	}
	familyID := uuid.New()
	access, err := s.tokens.IssueForSession(challenge.User.ID.String(), roles.Directory(userRoles), familyID.String())
	if err != nil {
		return Result{}, fmt.Errorf("issue mfa access token: %w", err)
	}
	refreshToken, err := s.createRefreshToken(ctx, w, challenge.User.ID, familyID, input, now)
	if err != nil {
		return Result{}, err
	}
	hubSessionToken, err := s.createHubSession(ctx, w, challenge.User.ID, familyID, now)
	if err != nil {
		return Result{}, err
	}
	// The login only succeeds here, once the second factor is proven (D11).
	if err := w.UpdateLastLogin(ctx, challenge.User.ID); err != nil {
		return Result{}, fmt.Errorf("update last login: %w", err)
	}
	for _, action := range []string{"mfa_code_accepted", "login_succeeded"} {
		if err := w.InsertAuditEvent(ctx, AuditEvent{ActorUserID: challenge.User.ID, Action: action, IP: input.IP, UserAgent: input.UserAgent}); err != nil {
			return Result{}, fmt.Errorf("audit %s: %w", action, err)
		}
	}
	return Result{AccessToken: access, RefreshToken: refreshToken, HubSessionToken: hubSessionToken, TokenType: "Bearer", ExpiresIn: token.AccessTokenExpiresIn}, nil
}

// createRefreshToken stores a new refresh token for the family and returns its
// encoded value.
func (s *Service) createRefreshToken(ctx context.Context, w Writer, userID, familyID uuid.UUID, input VerifyInput, now time.Time) (string, error) {
	refreshRaw := make([]byte, 32)
	if _, err := io.ReadFull(s.random, refreshRaw); err != nil {
		return "", fmt.Errorf("generate mfa refresh token: %w", err)
	}
	refreshHash := sha256.Sum256(refreshRaw)
	if err := w.CreateRefreshToken(ctx, RefreshToken{UserID: userID, TokenHash: refreshHash[:], FamilyID: familyID, IP: input.IP, UserAgent: input.UserAgent, ExpiresAt: now.Add(s.refreshTTL)}); err != nil {
		return "", fmt.Errorf("create mfa refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(refreshRaw), nil
}

// createHubSession stores a hub session for the family and returns its encoded
// token, or an empty token when hub sessions are disabled.
func (s *Service) createHubSession(ctx context.Context, w Writer, userID, familyID uuid.UUID, now time.Time) (string, error) {
	if s.hubSessionTTL <= 0 {
		return "", nil
	}
	hubRaw := make([]byte, 32)
	if _, err := io.ReadFull(s.random, hubRaw); err != nil {
		return "", fmt.Errorf("generate hub session token: %w", err)
	}
	hubHash := sha256.Sum256(hubRaw)
	if err := w.CreateHubSession(ctx, HubSession{UserID: userID, TokenHash: hubHash[:], FamilyID: familyID, ExpiresAt: now.Add(s.hubSessionTTL)}); err != nil {
		return "", fmt.Errorf("create hub session: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(hubRaw), nil
}

// finishVerify runs after the transaction commits: it publishes the lock event
// if any, then reports the domain error or the session.
func (s *Service) finishVerify(ctx context.Context, outcome verifyOutcome) (Result, error) {
	if outcome.lockEvent != nil {
		if err := s.publisher.Publish(ctx, events.TypeAccountLocked, *outcome.lockEvent); err != nil {
			// The lock is already committed and audited; the client still gets
			// the 401, so the failed notification is logged here (same shape as
			// the login publisher) and joined the way login.Service does.
			if s.logger != nil {
				s.logger.Warn("security event could not be published", "event_type", events.TypeAccountLocked, "user_id", outcome.lockEvent.Data.UserID, "error", err)
			}
			return Result{}, errors.Join(outcome.verificationErr, fmt.Errorf("publish account locked event: %w", err))
		}
	}
	if outcome.verificationErr != nil {
		return Result{}, outcome.verificationErr
	}
	return outcome.result, nil
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

// resendOutcome is what the resend transaction leaves behind for the code
// after commit: the event to publish and the values needed to restore the
// previous code if publishing fails.
type resendOutcome struct {
	event                  events.MfaChallengeIssued
	challengeID            uuid.UUID
	newHash, previousHash  []byte
	sentAt, previousSentAt time.Time
}

func (s *Service) Resend(ctx context.Context, encodedToken string) error {
	raw, err := base64.RawURLEncoding.DecodeString(encodedToken)
	if err != nil {
		return ErrChallengeInvalid
	}
	tokenHash := sha256.Sum256(raw)
	var outcome resendOutcome
	if err := s.repository.WithinMFATransaction(ctx, func(w Writer) error {
		return s.resendChallenge(ctx, w, raw, tokenHash[:], &outcome)
	}); err != nil {
		return err
	}
	if err := s.publishCode(ctx, outcome.event); err != nil {
		return s.restoreUndeliveredResend(ctx, outcome, err)
	}
	return nil
}

// resendChallenge runs inside the resend transaction: it checks that the
// challenge is open and past the resend interval, stores a fresh code hash and
// audits it, recording in outcome what the caller needs after commit.
func (s *Service) resendChallenge(ctx context.Context, w Writer, raw, tokenHash []byte, outcome *resendOutcome) error {
	challenge, err := w.GetChallengeForUpdate(ctx, tokenHash)
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
	outcome.challengeID, outcome.sentAt = challenge.ID, now
	outcome.newHash, outcome.previousHash = hashCode(raw, code), append([]byte(nil), challenge.CodeHash...)
	outcome.previousSentAt = challenge.LastSentAt
	if err := w.ResendChallenge(ctx, outcome.challengeID, outcome.newHash, outcome.sentAt); err != nil {
		return fmt.Errorf("resend mfa challenge: %w", err)
	}
	if err := w.InsertAuditEvent(ctx, AuditEvent{ActorUserID: challenge.User.ID, Action: "mfa_code_resent"}); err != nil {
		return fmt.Errorf("audit mfa resend: %w", err)
	}
	outcome.event = challengeEvent(challenge.User, code, challenge.ExpiresAt)
	return nil
}

// restoreUndeliveredResend puts the prior code and timestamp back after a
// failed publish and returns publishErr, or the infrastructure error when the
// restore fails too.
func (s *Service) restoreUndeliveredResend(ctx context.Context, outcome resendOutcome, publishErr error) error {
	compensationCtx, cancel := compensationContext(ctx)
	defer cancel()
	if restoreErr := s.repository.WithinMFATransaction(compensationCtx, func(w Writer) error {
		if err := w.RestoreResend(compensationCtx, outcome.challengeID, outcome.newHash, outcome.previousHash, outcome.sentAt, outcome.previousSentAt); err != nil {
			return fmt.Errorf("restore failed mfa resend: %w", err)
		}
		return nil
	}); restoreErr != nil {
		s.logCompensationFailure("restore_mfa_resend", restoreErr)
		// Do not keep ErrDeliveryUnavailable in this path: the compensating
		// database failure must surface as infrastructure failure (5xx).
		return fmt.Errorf("restore undelivered mfa resend: %w", restoreErr)
	}
	return publishErr
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
	rawToken, code, err := s.newIssueSecrets()
	if err != nil {
		return Challenge{}, err
	}
	tokenHash, codeHash := sha256.Sum256(rawToken), hashCode(rawToken, code)
	challengeID := uuid.New()
	now := s.now().UTC()
	expiresAt := now.Add(ChallengeTTL)
	params := CreateParams{ID: challengeID, UserID: user.ID, TokenHash: tokenHash[:], CodeHash: codeHash, ExpiresAt: expiresAt, AttemptsLeft: MaxAttempts, SentAt: now}
	if err := s.repository.WithinMFATransaction(ctx, func(w Writer) error {
		return s.storeChallenge(ctx, w, user, params, now)
	}); err != nil {
		return Challenge{}, err
	}
	if err := s.publishCode(ctx, challengeEvent(user, code, expiresAt)); err != nil {
		return Challenge{}, s.cancelUndelivered(ctx, challengeID, err)
	}
	return Challenge{Token: base64.RawURLEncoding.EncodeToString(rawToken), ExpiresIn: challengeExpiresIn}, nil
}

// newIssueSecrets draws the raw challenge token and then the six-digit code,
// in that order, from the service's random source.
func (s *Service) newIssueSecrets() ([]byte, string, error) {
	rawToken := make([]byte, 32)
	if _, err := io.ReadFull(s.random, rawToken); err != nil {
		return nil, "", fmt.Errorf("generate mfa token: %w", err)
	}
	code, err := generateCode(s.random)
	if err != nil {
		return nil, "", fmt.Errorf("generate mfa code: %w", err)
	}
	return rawToken, code, nil
}

// storeChallenge runs inside the issuance transaction: it enforces the
// per-window issuance cap, supersedes the account's open challenges and
// stores the new one with its audit event.
func (s *Service) storeChallenge(ctx context.Context, w Writer, user User, params CreateParams, now time.Time) error {
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
	if err := w.CreateChallenge(ctx, params); err != nil {
		return fmt.Errorf("create mfa challenge: %w", err)
	}
	if err := w.InsertAuditEvent(ctx, AuditEvent{ActorUserID: user.ID, Action: "mfa_challenge_issued"}); err != nil {
		return fmt.Errorf("audit mfa challenge issued: %w", err)
	}
	return nil
}

// cancelUndelivered deletes a challenge whose code could not be published and
// returns publishErr, or the infrastructure error when the deletion fails too.
func (s *Service) cancelUndelivered(ctx context.Context, challengeID uuid.UUID, publishErr error) error {
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
		return fmt.Errorf("cancel undelivered mfa challenge: %w", cancelErr)
	}
	return publishErr
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
