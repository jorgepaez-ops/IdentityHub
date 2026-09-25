// Package verification implements the one-time email-verification use case.
package verification

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

var (
	ErrTokenInvalid = errors.New("email verification token is invalid")
	ErrPublish      = errors.New("publish email verified event")
	ErrInvalidInput = errors.New("email verification token is required")
)

type Publisher interface {
	Publish(context.Context, string, any) error
}

type Repository interface {
	WithinEmailVerificationTransaction(context.Context, func(store.EmailVerificationWriter) error) error
}

type Input struct {
	Token     string
	IP        *netip.Addr
	UserAgent *string
}

type Service struct {
	repository Repository
	publisher  Publisher
}

func New(repository Repository, publisher Publisher) *Service {
	return &Service{repository: repository, publisher: publisher}
}

// Verify atomically consumes the token, activates its account, records the
// audit event, and publishes the notification only after broker confirmation.
func (s *Service) Verify(ctx context.Context, input Input) error {
	if strings.TrimSpace(input.Token) == "" {
		return ErrInvalidInput
	}
	tokenHash := sha256.Sum256([]byte(input.Token))
	return s.repository.WithinEmailVerificationTransaction(ctx, func(writer store.EmailVerificationWriter) error {
		user, err := writer.ConsumeEmailVerificationToken(ctx, tokenHash[:])
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, ErrTokenInvalid) {
				return ErrTokenInvalid
			}
			return fmt.Errorf("consume email verification token: %w", err)
		}
		resourceType, resourceID := "user", user.ID.String()
		if _, err := writer.InsertAuditEvent(ctx, store.InsertAuditEventParams{
			ActorUserID:  &user.ID,
			Action:       "email_verified",
			ResourceType: &resourceType,
			ResourceID:   &resourceID,
			IP:           input.IP,
			UserAgent:    input.UserAgent,
			Metadata:     []byte(`{}`),
		}); err != nil {
			return fmt.Errorf("record email verification audit: %w", err)
		}
		event := events.EmailVerified{Envelope: events.NewEnvelope(events.TypeEmailVerified, "")}
		event.Data.UserID, event.Data.Email, event.Data.DisplayName = user.ID, user.Email, user.DisplayName
		if err := s.publisher.Publish(ctx, events.TypeEmailVerified, event); err != nil {
			return fmt.Errorf("%w: %w", ErrPublish, err)
		}
		return nil
	})
}

// Verifier is the narrow API boundary for email verification.
type Verifier interface {
	Verify(context.Context, Input) error
}
