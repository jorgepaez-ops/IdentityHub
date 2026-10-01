// Package invitation implements the one-time invitation-acceptance use case
// (T5, RF-002). It uses a hash-only, single-use token-consumption pattern and
// sets the Argon2id password hash the invitee chooses, since D9 folded email
// verification into invitation acceptance.
package invitation

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

var (
	ErrTokenInvalid = errors.New("invitation token is invalid")
	ErrPublish      = errors.New("publish account activated event")
	ErrInvalidInput = errors.New("invitation token is required")
)

type Hasher interface {
	Hash(password string) (string, error)
}
type Publisher interface {
	Publish(context.Context, string, any) error
}
type Repository interface {
	InvitationTokenIsUsable(context.Context, []byte) (bool, error)
	WithinInvitationAcceptanceTransaction(context.Context, func(store.InvitationAcceptanceWriter) error) error
}

type Input struct {
	Token, Password string
	IP              *netip.Addr
	UserAgent       *string
}

type InvalidInputError struct{ Field, Detail string }

func (e *InvalidInputError) Error() string { return e.Field + ": " + e.Detail }

type Service struct {
	repository Repository
	publisher  Publisher
	hasher     Hasher
}

func New(repository Repository, publisher Publisher, hasher Hasher) *Service {
	return &Service{repository: repository, publisher: publisher, hasher: hasher}
}

// Accept atomically consumes the invitation token, sets its account's
// Argon2id password hash, activates the account, records the audit event,
// and publishes the same user.email_verified notification email
// verification used to (D9: accepting the invitation supersedes it), only
// after the broker confirms the publish.
func (s *Service) Accept(ctx context.Context, input Input) error {
	if strings.TrimSpace(input.Token) == "" {
		return ErrInvalidInput
	}
	if n := utf8.RuneCountInString(input.Password); n < password.AccountPasswordMinRunes || n > password.AccountPasswordMaxRunes {
		return &InvalidInputError{Field: "password", Detail: "must contain 12 to 128 characters"}
	}
	// employee.Service emits the raw token bytes base64url-encoded (see
	// employee.go); hash the same raw bytes here, never the encoded string,
	// or a real emailed link can never match what was stored.
	raw, err := base64.RawURLEncoding.DecodeString(input.Token)
	if err != nil {
		return ErrTokenInvalid
	}
	tokenHash := sha256.Sum256(raw)
	usable, err := s.repository.InvitationTokenIsUsable(ctx, tokenHash[:])
	if err != nil {
		return err
	}
	if !usable {
		return ErrTokenInvalid
	}
	passwordHash, err := s.hasher.Hash(input.Password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.repository.WithinInvitationAcceptanceTransaction(ctx, func(writer store.InvitationAcceptanceWriter) error {
		user, err := writer.ConsumeInvitationToken(ctx, store.ConsumeInvitationTokenParams{TokenHash: tokenHash[:], PasswordHash: passwordHash})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrTokenInvalid) {
				return ErrTokenInvalid
			}
			return fmt.Errorf("consume invitation token: %w", err)
		}
		resourceType, resourceID := "user", user.ID.String()
		if _, err := writer.InsertAuditEvent(ctx, store.InsertAuditEventParams{
			ActorUserID:  &user.ID,
			Action:       "invitation_accepted",
			ResourceType: &resourceType,
			ResourceID:   &resourceID,
			IP:           input.IP,
			UserAgent:    input.UserAgent,
			Metadata:     []byte(`{}`),
		}); err != nil {
			return fmt.Errorf("record invitation accepted audit: %w", err)
		}
		event := events.EmailVerified{Envelope: events.NewEnvelope(events.TypeEmailVerified, "")}
		event.Data.UserID, event.Data.Email, event.Data.DisplayName = user.ID, user.Email, user.DisplayName
		if err := s.publisher.Publish(ctx, events.TypeEmailVerified, event); err != nil {
			return fmt.Errorf("%w: %w", ErrPublish, err)
		}
		return nil
	})
}

// Acceptor is the narrow API boundary for invitation acceptance.
type Acceptor interface {
	Accept(context.Context, Input) error
}
