package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jorgepaez/identity-hub/internal/auth/session"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

func (s *Store) ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]session.Session, error) {
	rows, err := s.queries.ListActiveRefreshSessions(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list active refresh sessions: %w", err)
	}
	items := make([]session.Session, 0, len(rows))
	for _, row := range rows {
		items = append(items, session.Session{
			ID: row.FamilyID, IP: row.Ip, UserAgent: row.UserAgent.String,
			CreatedAt: row.CreatedAt.Time, LastUsedAt: row.LastUsedAt.Time,
		})
	}
	return items, nil
}

func (s *Store) WithinSessionTransaction(ctx context.Context, fn func(session.Writer) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin session transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(&sessionWriter{queries: generated.New(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit session transaction: %w", err)
	}
	return nil
}

type sessionWriter struct{ queries *generated.Queries }

func (w *sessionWriter) RevokeActiveSession(ctx context.Context, userID, familyID uuid.UUID) (bool, error) {
	revoked, err := w.queries.RevokeActiveRefreshSession(ctx, generated.RevokeActiveRefreshSessionParams{UserID: userID, FamilyID: familyID})
	if err != nil {
		return false, fmt.Errorf("revoke active refresh session: %w", err)
	}
	return revoked, nil
}

func (w *sessionWriter) InsertAuditEvent(ctx context.Context, event session.AuditEvent) error {
	metadata, err := json.Marshal(map[string]string{})
	if err != nil {
		return fmt.Errorf("marshal session audit metadata: %w", err)
	}
	if _, err := w.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{
		ActorUserID: nullableUUID(event.ActorUserID), Action: event.Action,
		ResourceType: pgtype.Text{String: event.ResourceType, Valid: true},
		ResourceID:   pgtype.Text{String: event.ResourceID, Valid: true},
		Metadata:     metadata,
	}); err != nil {
		return fmt.Errorf("insert session audit event: %w", err)
	}
	return nil
}
