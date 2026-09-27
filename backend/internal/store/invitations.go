package store

import (
	"context"
	"fmt"

	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

// ConsumeInvitationTokenParams contains the invitation token hash and the
// Argon2id hash of the password the invitee just chose (T5, RF-002).
type ConsumeInvitationTokenParams struct {
	TokenHash    []byte
	PasswordHash string
}

// InvitationAcceptanceWriter groups the writes that must succeed atomically
// when a one-time invitation token is consumed: setting the password,
// activating the account, and recording the audit event.
type InvitationAcceptanceWriter interface {
	ConsumeInvitationToken(context.Context, ConsumeInvitationTokenParams) (User, error)
	InsertAuditEvent(context.Context, InsertAuditEventParams) (AuditEvent, error)
}

// InvitationTokenIsUsable cheaply checks whether an invitation can still be
// accepted. The subsequent consume operation remains authoritative because the
// token may change state between this read and the transaction.
func (s *Store) InvitationTokenIsUsable(ctx context.Context, tokenHash []byte) (bool, error) {
	usable, err := s.queries.InvitationTokenIsUsable(ctx, tokenHash)
	if err != nil {
		return false, fmt.Errorf("check invitation token: %w", err)
	}
	return usable, nil
}

// WithinInvitationAcceptanceTransaction runs invitation-acceptance writes in
// one PostgreSQL transaction.
func (s *Store) WithinInvitationAcceptanceTransaction(ctx context.Context, fn func(InvitationAcceptanceWriter) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin invitation acceptance transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	writer := &invitationAcceptanceWriter{queries: generated.New(tx)}
	if err := fn(writer); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit invitation acceptance transaction: %w", err)
	}
	return nil
}

type invitationAcceptanceWriter struct{ queries *generated.Queries }

func (w *invitationAcceptanceWriter) ConsumeInvitationToken(ctx context.Context, params ConsumeInvitationTokenParams) (User, error) {
	user, err := w.queries.ConsumeInvitationToken(ctx, generated.ConsumeInvitationTokenParams{TokenHash: params.TokenHash, PasswordHash: params.PasswordHash})
	if err != nil {
		return User{}, fmt.Errorf("consume invitation token: %w", err)
	}
	return userFromGenerated(user), nil
}

func (w *invitationAcceptanceWriter) InsertAuditEvent(ctx context.Context, params InsertAuditEventParams) (AuditEvent, error) {
	event, err := w.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{ActorUserID: nullableUUID(params.ActorUserID), Action: params.Action, ResourceType: nullableText(params.ResourceType), ResourceID: nullableText(params.ResourceID), Ip: copyAddr(params.IP), UserAgent: nullableText(params.UserAgent), Metadata: params.Metadata})
	if err != nil {
		return AuditEvent{}, fmt.Errorf("insert audit event: %w", err)
	}
	return auditEventFromGenerated(event), nil
}
