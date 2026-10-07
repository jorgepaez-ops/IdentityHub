// Package passwordreset implements RF-015 with hash-only, expiring reset
// tokens. It shares the account-token table with invitations but keeps the
// password_reset purpose independent from invitation tokens.
package passwordreset

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

var (
	ErrTokenInvalid  = errors.New("password reset token is invalid")
	ErrTokenRequired = errors.New("password reset token is required")
	ErrPublish       = errors.New("publish password reset event")
)

const resetTTL = time.Hour

type Hasher interface{ Hash(string) (string, error) }
type Publisher interface {
	Publish(context.Context, string, any) error
}
type Repository interface {
	PasswordResetTokenIsUsable(context.Context, []byte) (bool, error)
	WithinPasswordResetRequestTransaction(context.Context, func(store.PasswordResetRequestWriter) error) error
	WithinPasswordResetConfirmationTransaction(context.Context, func(store.PasswordResetConfirmationWriter) error) error
}

type Service struct {
	repository Repository
	publisher  Publisher
	hasher     Hasher
	random     io.Reader
	now        func() time.Time
	logger     *slog.Logger
}

// WithLogger sets where a failed post-commit alert (D16) is reported. Nil
// disables logging.
func (s *Service) WithLogger(logger *slog.Logger) *Service {
	s.logger = logger
	return s
}

func New(repository Repository, publisher Publisher, hasher Hasher, random io.Reader, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repository: repository, publisher: publisher, hasher: hasher, random: random, now: now}
}

// Request always performs the same token generation, SHA-256, and parameterized
// INSERT ... SELECT transaction. The query only persists the hash if email is
// registered. Both paths publish the same event, while its broker-only
// accountExists field prevents SMTP delivery for a missing account.
func (s *Service) Request(ctx context.Context, email string) error {
	raw := make([]byte, 32)
	if _, err := io.ReadFull(s.random, raw); err != nil {
		return fmt.Errorf("generate password reset token: %w", err)
	}
	tokenHash := sha256.Sum256(raw)
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	expiresAt := s.now().UTC().Add(resetTTL)
	return s.repository.WithinPasswordResetRequestTransaction(ctx, func(writer store.PasswordResetRequestWriter) error {
		accountExists, err := writer.CreatePasswordResetTokenForEmail(ctx, normalizedEmail, store.CreatePasswordResetTokenParams{TokenHash: tokenHash[:], ExpiresAt: expiresAt})
		if err != nil {
			return fmt.Errorf("persist password reset token: %w", err)
		}
		event := events.PasswordResetRequested{Envelope: events.NewEnvelope(events.TypePasswordResetRequested, "")}
		event.Data.Email, event.Data.ResetToken, event.Data.ExpiresAt, event.Data.AccountExists = normalizedEmail, base64.RawURLEncoding.EncodeToString(raw), expiresAt, accountExists
		if err := s.publisher.Publish(ctx, events.TypePasswordResetRequested, event); err != nil {
			return fmt.Errorf("%w: %w", ErrPublish, err)
		}
		return nil
	})
}

// decodeConfirmInput validates the request before any database work, in this
// order: the token is present, the password length is within policy and the
// token decodes. It returns the raw token bytes.
func decodeConfirmInput(token, newPassword string) ([]byte, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrTokenRequired
	}
	if n := utf8.RuneCountInString(newPassword); n < password.AccountPasswordMinRunes || n > password.AccountPasswordMaxRunes {
		return nil, &InvalidInputError{Field: "password", Detail: fmt.Sprintf("must contain %d to %d characters", password.AccountPasswordMinRunes, password.AccountPasswordMaxRunes)}
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, ErrTokenInvalid
	}
	return raw, nil
}

// confirmOutcome is what the confirmation transaction leaves behind for the
// alert published after commit.
type confirmOutcome struct {
	user     store.User
	unlocked bool
}

func (s *Service) Confirm(ctx context.Context, token, newPassword string) error {
	raw, err := decodeConfirmInput(token, newPassword)
	if err != nil {
		return err
	}
	tokenHash := sha256.Sum256(raw)
	usable, err := s.repository.PasswordResetTokenIsUsable(ctx, tokenHash[:])
	if err != nil {
		return err
	}
	if !usable {
		return ErrTokenInvalid
	}
	passwordHash, err := s.hasher.Hash(newPassword)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	var outcome confirmOutcome
	if err := s.repository.WithinPasswordResetConfirmationTransaction(ctx, func(writer store.PasswordResetConfirmationWriter) error {
		return s.consumeReset(ctx, writer, tokenHash[:], passwordHash, &outcome)
	}); err != nil {
		return err
	}
	s.publishResetCompleted(ctx, outcome)
	return nil
}

// consumeReset runs inside the confirmation transaction: it consumes the token,
// revokes the sessions and audits the completed reset, recording the user and
// the unlock flag in outcome.
func (s *Service) consumeReset(ctx context.Context, writer store.PasswordResetConfirmationWriter, tokenHash []byte, passwordHash string, outcome *confirmOutcome) error {
	var err error
	outcome.user, outcome.unlocked, err = writer.ConsumePasswordResetTokenAndRevokeSessions(ctx, store.ConsumePasswordResetTokenParams{TokenHash: tokenHash, PasswordHash: passwordHash})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrTokenInvalid
		}
		return fmt.Errorf("consume password reset token: %w", err)
	}
	// D13: every completed reset is audited, with whether it also lifted
	// an RF-017 lockout, regardless of the account's previous status.
	metadata, err := json.Marshal(struct {
		Unlocked bool `json:"unlocked"`
	}{Unlocked: outcome.unlocked})
	if err != nil {
		return fmt.Errorf("marshal password reset completed metadata: %w", err)
	}
	resourceType, resourceID := "user", outcome.user.ID.String()
	if _, err := writer.InsertAuditEvent(ctx, store.InsertAuditEventParams{
		ActorUserID: &outcome.user.ID,
		// Must remain identical to the literal in the CountLoginFailuresByAccount
		// query (db/queries/audit.sql): the D13 reset boundary depends on it.
		Action:       "password_reset_completed",
		ResourceType: &resourceType,
		ResourceID:   &resourceID,
		Metadata:     metadata,
	}); err != nil {
		return fmt.Errorf("record password reset completed audit: %w", err)
	}
	return nil
}

// publishResetCompleted sends the completed-reset alert after the commit.
// D14: deliver this alert for every completed reset, not just when D13
// also unlocks the account. It contains no credential material.
// D16: it goes out only after the commit. The reset is already durable, so
// a broker failure is logged and the caller still gets success: failing
// here would block account recovery (including the D13 unlock) on the
// broker, and a retry would find the token consumed.
func (s *Service) publishResetCompleted(ctx context.Context, outcome confirmOutcome) {
	user := outcome.user
	event := events.PasswordResetCompleted{Envelope: events.NewEnvelope(events.TypePasswordResetCompleted, "")}
	event.Data.UserID, event.Data.Email, event.Data.DisplayName, event.Data.Unlocked = user.ID, user.Email, user.DisplayName, outcome.unlocked
	if err := s.publisher.Publish(ctx, events.TypePasswordResetCompleted, event); err != nil && s.logger != nil {
		s.logger.Error("password reset completed alert could not be published", "event_type", events.TypePasswordResetCompleted, "user_id", user.ID, "error", err)
	}
}

type InvalidInputError struct{ Field, Detail string }

func (e *InvalidInputError) Error() string { return e.Field + ": " + e.Detail }

type Requester interface {
	Request(context.Context, string) error
}
type Confirmer interface {
	Confirm(context.Context, string, string) error
}

type HandlerService interface {
	Requester
	Confirmer
}
