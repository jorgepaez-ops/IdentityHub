// Package session manages the refresh-token families that represent a user's active sessions.
package session

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
)

var ErrSessionNotFound = errors.New("session not found")

// Session is the safe, family-level view of one active refresh session.
type Session struct {
	ID                    uuid.UUID
	IP                    *netip.Addr
	UserAgent             string
	CreatedAt, LastUsedAt time.Time
}

type AuditEvent struct {
	ActorUserID              *uuid.UUID
	Action                   string
	ResourceType, ResourceID string
}

type Writer interface {
	RevokeActiveSession(context.Context, uuid.UUID, uuid.UUID) (bool, error)
	InsertAuditEvent(context.Context, AuditEvent) error
}

type Repository interface {
	ListActiveSessions(context.Context, uuid.UUID) ([]Session, error)
	WithinSessionTransaction(context.Context, func(Writer) error) error
}

type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]Session, error) {
	if s.repository == nil {
		return nil, fmt.Errorf("session service is unavailable")
	}
	items, err := s.repository.ListActiveSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	return items, nil
}

// Revoke revokes only an active refresh family owned by the authenticated user
// and records the security-relevant action in the same transaction.
func (s *Service) Revoke(ctx context.Context, userID, familyID uuid.UUID) error {
	if s.repository == nil {
		return fmt.Errorf("session service is unavailable")
	}
	return s.repository.WithinSessionTransaction(ctx, func(writer Writer) error {
		revoked, err := writer.RevokeActiveSession(ctx, userID, familyID)
		if err != nil {
			return fmt.Errorf("revoke active session: %w", err)
		}
		if !revoked {
			return ErrSessionNotFound
		}
		if err := writer.InsertAuditEvent(ctx, AuditEvent{
			ActorUserID:  &userID,
			Action:       "session_revoked",
			ResourceType: "refresh_session",
			ResourceID:   familyID.String(),
		}); err != nil {
			return fmt.Errorf("audit session revocation: %w", err)
		}
		return nil
	})
}
