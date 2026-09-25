// Package logout revokes the presented refresh token.
package logout

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/netip"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var ErrInvalidRefreshToken = errors.New("invalid refresh token")

type Input struct {
	RefreshToken string
	IP           *netip.Addr
	UserAgent    *string
}

type AuditEvent struct {
	ActorUserID *uuid.UUID
	Action      string
	Reason      string
	IP          *netip.Addr
	UserAgent   *string
}

type Writer interface {
	RevokeRefreshToken(context.Context, []byte) (uuid.UUID, error)
	InsertAuditEvent(context.Context, AuditEvent) error
}

type Repository interface {
	WithinLogoutTransaction(context.Context, func(Writer) error) error
}

type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) Logout(ctx context.Context, input Input) error {
	if s.repository == nil || input.RefreshToken == "" {
		return ErrInvalidRefreshToken
	}
	tokenHash := sha256.Sum256([]byte(input.RefreshToken))
	return s.repository.WithinLogoutTransaction(ctx, func(writer Writer) error {
		userID, err := writer.RevokeRefreshToken(ctx, tokenHash[:])
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrInvalidRefreshToken
			}
			return fmt.Errorf("revoke refresh token: %w", err)
		}
		if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: &userID, Action: "logout", Reason: "refresh_token_revoked", IP: input.IP, UserAgent: input.UserAgent}); err != nil {
			return fmt.Errorf("record logout audit: %w", err)
		}
		return nil
	})
}
