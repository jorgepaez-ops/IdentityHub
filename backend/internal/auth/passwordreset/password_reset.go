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

func (s *Service) Confirm(ctx context.Context, token, newPassword string) error {
	if strings.TrimSpace(token) == "" {
		return ErrTokenRequired
	}
	if n := utf8.RuneCountInString(newPassword); n < password.AccountPasswordMinRunes || n > password.AccountPasswordMaxRunes {
		return &InvalidInputError{Field: "password", Detail: "must contain 12 to 128 characters"}
	}
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return ErrTokenInvalid
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
	return s.repository.WithinPasswordResetConfirmationTransaction(ctx, func(writer store.PasswordResetConfirmationWriter) error {
		user, unlocked, err := writer.ConsumePasswordResetTokenAndRevokeSessions(ctx, store.ConsumePasswordResetTokenParams{TokenHash: tokenHash[:], PasswordHash: passwordHash})
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
		}{Unlocked: unlocked})
		if err != nil {
			return fmt.Errorf("marshal password reset completed metadata: %w", err)
		}
		resourceType, resourceID := "user", user.ID.String()
		if _, err := writer.InsertAuditEvent(ctx, store.InsertAuditEventParams{
			ActorUserID:  &user.ID,
			Action:       "password_reset_completed",
			ResourceType: &resourceType,
			ResourceID:   &resourceID,
			Metadata:     metadata,
		}); err != nil {
			return fmt.Errorf("record password reset completed audit: %w", err)
		}
		return nil
	})
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
