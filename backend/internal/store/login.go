package store

import (
	"context"
	"encoding/json"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

// WithinLoginTransaction groups the password check, its transparent rehash and
// the failure audit/lockout bookkeeping. A failed login audit is also committed as the callback returns
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
	row, err := w.queries.GetLoginUserByEmail(ctx, email)
	if err != nil {
		return login.User{}, fmt.Errorf("get login user by email: %w", err)
	}
	return login.User{ID: row.ID, Email: row.Email, DisplayName: row.DisplayName, PasswordHash: row.PasswordHash, Status: login.Status(row.Status), LockedUntil: optionalTime(row.LockedUntil)}, nil
}

func (w *loginWriter) CountLoginFailuresByAccount(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	count, err := w.queries.CountLoginFailuresByAccount(ctx, generated.CountLoginFailuresByAccountParams{ActorUserID: nullableUUID(&userID), CreatedAt: pgtype.Timestamptz{Time: since, Valid: true}})
	if err != nil {
		return 0, fmt.Errorf("count login failures by account: %w", err)
	}
	return count, nil
}

func (w *loginWriter) CountLoginFailuresByIP(ctx context.Context, ip netip.Addr, since time.Time) (int64, error) {
	count, err := w.queries.CountLoginFailuresByIP(ctx, generated.CountLoginFailuresByIPParams{Ip: &ip, CreatedAt: pgtype.Timestamptz{Time: since, Valid: true}})
	if err != nil {
		return 0, fmt.Errorf("count login failures by IP: %w", err)
	}
	return count, nil
}

func (w *loginWriter) LockLoginUser(ctx context.Context, userID uuid.UUID, lockedUntil time.Time) error {
	if err := w.queries.LockLoginUser(ctx, generated.LockLoginUserParams{ID: userID, LockedUntil: pgtype.Timestamptz{Time: lockedUntil, Valid: true}}); err != nil {
		return fmt.Errorf("lock login user: %w", err)
	}
	return nil
}

func (w *loginWriter) UnlockLoginUser(ctx context.Context, userID uuid.UUID) error {
	if err := w.queries.UnlockLoginUser(ctx, userID); err != nil {
		return fmt.Errorf("unlock login user: %w", err)
	}
	return nil
}

func (w *loginWriter) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	if err := w.queries.UpdatePasswordHash(ctx, generated.UpdatePasswordHashParams{ID: userID, PasswordHash: passwordHash}); err != nil {
		return fmt.Errorf("update password hash: %w", err)
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
