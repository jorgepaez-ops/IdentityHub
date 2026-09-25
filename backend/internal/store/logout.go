package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jorgepaez/identity-hub/internal/auth/logout"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

// WithinLogoutTransaction groups single-token revocation and its audit event.
func (s *Store) WithinLogoutTransaction(ctx context.Context, fn func(logout.Writer) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin logout transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(&logoutWriter{queries: generated.New(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit logout transaction: %w", err)
	}
	return nil
}

type logoutWriter struct{ queries *generated.Queries }

func (w *logoutWriter) RevokeRefreshToken(ctx context.Context, tokenHash []byte) (uuid.UUID, error) {
	userID, err := w.queries.RevokeRefreshToken(ctx, tokenHash)
	if err != nil {
		return uuid.Nil, fmt.Errorf("revoke refresh token: %w", err)
	}
	return userID, nil
}

func (w *logoutWriter) InsertAuditEvent(ctx context.Context, event logout.AuditEvent) error {
	metadata, err := json.Marshal(map[string]string{"reason": event.Reason})
	if err != nil {
		return fmt.Errorf("marshal logout audit metadata: %w", err)
	}
	if _, err := w.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{ActorUserID: nullableUUID(event.ActorUserID), Action: event.Action, ResourceType: pgtype.Text{}, ResourceID: pgtype.Text{}, Ip: copyAddr(event.IP), UserAgent: nullableText(event.UserAgent), Metadata: metadata}); err != nil {
		return fmt.Errorf("insert logout audit event: %w", err)
	}
	return nil
}
