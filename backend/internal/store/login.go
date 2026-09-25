package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

// WithinLoginTransaction groups password rehashing, session issuance, and its
// audit event. A failed login audit is also committed as the callback returns
// the domain authentication error only after the audit insert succeeds.
func (s *Store) WithinLoginTransaction(ctx context.Context, fn func(login.Writer) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin login transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(&loginWriter{queries: generated.New(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit login transaction: %w", err)
	}
	return nil
}

type loginWriter struct{ queries *generated.Queries }

func (w *loginWriter) GetLoginUserByEmail(ctx context.Context, email string) (login.User, error) {
	user, err := w.queries.GetLoginUserByEmail(ctx, email)
	if err != nil {
		return login.User{}, fmt.Errorf("get login user by email: %w", err)
	}
	return login.User{ID: user.ID, Email: user.Email, PasswordHash: user.PasswordHash, Status: login.Status(user.Status), MFAEnabled: user.MfaEnabled}, nil
}

func (w *loginWriter) ListRolesForUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	roles, err := w.queries.ListRolesForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list roles for user: %w", err)
	}
	return roles, nil
}

func (w *loginWriter) UpdateLoginSuccess(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	if err := w.queries.UpdateLoginSuccess(ctx, generated.UpdateLoginSuccessParams{ID: userID, PasswordHash: passwordHash}); err != nil {
		return fmt.Errorf("update login success: %w", err)
	}
	return nil
}

func (w *loginWriter) CreateRefreshToken(ctx context.Context, refresh login.RefreshToken) error {
	if err := w.queries.CreateRefreshToken(ctx, generated.CreateRefreshTokenParams{UserID: refresh.UserID, TokenHash: refresh.TokenHash, FamilyID: refresh.FamilyID, Ip: copyAddr(refresh.IP), UserAgent: nullableText(refresh.UserAgent), ExpiresAt: pgtype.Timestamptz{Time: refresh.ExpiresAt, Valid: true}}); err != nil {
		return fmt.Errorf("create refresh token: %w", err)
	}
	return nil
}

func (w *loginWriter) InsertAuditEvent(ctx context.Context, event login.AuditEvent) error {
	metadata, err := json.Marshal(map[string]string{"reason": event.Reason})
	if err != nil {
		return fmt.Errorf("marshal login audit metadata: %w", err)
	}
	if _, err := w.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{ActorUserID: nullableUUID(event.ActorUserID), Action: event.Action, ResourceType: pgtype.Text{}, ResourceID: pgtype.Text{}, Ip: copyAddr(event.IP), UserAgent: nullableText(event.UserAgent), Metadata: metadata}); err != nil {
		return fmt.Errorf("insert login audit event: %w", err)
	}
	return nil
}
