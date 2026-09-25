package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jorgepaez/identity-hub/internal/auth/refresh"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

// WithinRefreshTransaction groups rotation and reuse revocation so concurrent
// requests cannot leave a family with more than one active token.
func (s *Store) WithinRefreshTransaction(ctx context.Context, fn func(refresh.Writer) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin refresh transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(&refreshWriter{queries: generated.New(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit refresh transaction: %w", err)
	}
	return nil
}

type refreshWriter struct{ queries *generated.Queries }

func (w *refreshWriter) RotateRefreshToken(ctx context.Context, tokenHash []byte, next refresh.RefreshToken) (refresh.Rotation, error) {
	row, err := w.queries.RotateRefreshToken(ctx, generated.RotateRefreshTokenParams{
		TokenHash: tokenHash, TokenHash_2: next.TokenHash, Ip: copyAddr(next.IP), UserAgent: nullableText(next.UserAgent), ExpiresAt: pgtype.Timestamptz{Time: next.ExpiresAt, Valid: true},
	})
	if err != nil {
		return refresh.Rotation{}, fmt.Errorf("rotate refresh token: %w", err)
	}
	rotation := refresh.Rotation{UserID: row.UserID, FamilyID: row.FamilyID, ParentID: row.ParentID}
	switch {
	case row.Rotated:
		rotation.Status = refresh.RotationSucceeded
	case row.Status == generated.RefreshStatusRotated:
		rotation.Status = refresh.RotationReused
	default:
		rotation.Status = refresh.RotationInvalid
	}
	return rotation, nil
}

func (w *refreshWriter) RevokeRefreshFamily(ctx context.Context, familyID uuid.UUID) (int64, error) {
	count, err := w.queries.RevokeRefreshFamily(ctx, familyID)
	if err != nil {
		return 0, fmt.Errorf("revoke refresh family: %w", err)
	}
	return count, nil
}

func (w *refreshWriter) ListRolesForUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	roles, err := w.queries.ListRolesForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list roles for refresh user: %w", err)
	}
	return roles, nil
}

func (w *refreshWriter) InsertAuditEvent(ctx context.Context, event refresh.AuditEvent) error {
	metadata, err := json.Marshal(map[string]string{"reason": event.Reason})
	if err != nil {
		return fmt.Errorf("marshal refresh audit metadata: %w", err)
	}
	if _, err := w.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{ActorUserID: nullableUUID(event.ActorUserID), Action: event.Action, ResourceType: pgtype.Text{}, ResourceID: pgtype.Text{}, Ip: copyAddr(event.IP), UserAgent: nullableText(event.UserAgent), Metadata: metadata}); err != nil {
		return fmt.Errorf("insert refresh audit event: %w", err)
	}
	return nil
}
