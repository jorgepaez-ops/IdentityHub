// Package invitationresend implements D12: a privileged replacement
// invitation for a pending account whose original link was lost or expired.
package invitationresend

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

var (
	ErrNotFound          = errors.New("invitation account not found")
	ErrAccountNotPending = errors.New("invitation account is not pending verification")
	ErrPublish           = errors.New("publish replacement invitation event")
)

const invitationTTL = 24 * time.Hour

type Publisher interface {
	Publish(context.Context, string, any) error
}
type Repository interface {
	WithinInvitationResendTransaction(context.Context, func(store.InvitationResendWriter) error) error
}
type Input struct {
	ActorUserID, UserID uuid.UUID
	IP                  *netip.Addr
	UserAgent           *string
}
type Service struct {
	repository Repository
	publisher  Publisher
	random     io.Reader
	now        func() time.Time
}

func New(repository Repository, publisher Publisher, random io.Reader, now func() time.Time) *Service {
	if now == nil {
		now = time.Now
	}
	return &Service{repository, publisher, random, now}
}
func (s *Service) Resend(ctx context.Context, input Input) error {
	return s.repository.WithinInvitationResendTransaction(ctx, func(writer store.InvitationResendWriter) error {
		user, err := loadPendingUser(ctx, writer, input.UserID)
		if err != nil {
			return err
		}
		raw, expiresAt, err := s.replaceToken(ctx, writer, user.ID)
		if err != nil {
			return err
		}
		return s.auditAndPublish(ctx, writer, input, user, raw, expiresAt)
	})
}

// loadPendingUser locks the account and requires it to still be pending
// verification.
func loadPendingUser(ctx context.Context, writer store.InvitationResendWriter, userID uuid.UUID) (store.User, error) {
	user, err := writer.GetUserByIDForUpdate(ctx, userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.User{}, ErrNotFound
	}
	if err != nil {
		return store.User{}, fmt.Errorf("load invitation account: %w", err)
	}
	if user.Status != "pending_verification" {
		return store.User{}, ErrAccountNotPending
	}
	return user, nil
}

// replaceToken invalidates the old tokens and persists a new one, returning the
// raw token bytes and its expiry.
func (s *Service) replaceToken(ctx context.Context, writer store.InvitationResendWriter, userID uuid.UUID) ([]byte, time.Time, error) {
	if err := writer.InvalidateInvitationTokens(ctx, userID); err != nil {
		return nil, time.Time{}, fmt.Errorf("invalidate old invitation tokens: %w", err)
	}
	raw := make([]byte, 32)
	if _, err := io.ReadFull(s.random, raw); err != nil {
		return nil, time.Time{}, fmt.Errorf("generate replacement invitation token: %w", err)
	}
	hash := sha256.Sum256(raw)
	expiresAt := s.now().UTC().Add(invitationTTL)
	if err := writer.CreateInvitationToken(ctx, store.CreateInvitationTokenParams{UserID: userID, TokenHash: hash[:], ExpiresAt: expiresAt}); err != nil {
		return nil, time.Time{}, fmt.Errorf("persist replacement invitation token: %w", err)
	}
	return raw, expiresAt, nil
}

// auditAndPublish records the resend audit event and publishes the
// replacement invitation.
func (s *Service) auditAndPublish(ctx context.Context, writer store.InvitationResendWriter, input Input, user store.User, raw []byte, expiresAt time.Time) error {
	resourceType, resourceID := "user", user.ID.String()
	if _, err := writer.InsertAuditEvent(ctx, store.InsertAuditEventParams{ActorUserID: &input.ActorUserID, Action: "invitation_resent", ResourceType: &resourceType, ResourceID: &resourceID, IP: input.IP, UserAgent: input.UserAgent, Metadata: []byte(`{}`)}); err != nil {
		return fmt.Errorf("record invitation resend audit: %w", err)
	}
	event := events.UserInvited{Envelope: events.NewEnvelope(events.TypeUserInvited, "")}
	event.Data.UserID, event.Data.Email, event.Data.DisplayName, event.Data.InvitationToken, event.Data.ExpiresAt = user.ID, user.Email, user.DisplayName, base64.RawURLEncoding.EncodeToString(raw), expiresAt
	if err := s.publisher.Publish(ctx, events.TypeUserInvited, event); err != nil {
		return fmt.Errorf("%w: %w", ErrPublish, err)
	}
	return nil
}

type Resender interface {
	Resend(context.Context, Input) error
}
