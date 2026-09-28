package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jorgepaez/identity-hub/internal/auth/mfa"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

// WithinTransaction serializes challenge creation and its immutable audit
// record. The challenge secret itself is never persisted.
func (s *Store) WithinTransaction(ctx context.Context, fn func(mfa.Writer) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin mfa transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(&mfaWriter{queries: generated.New(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit mfa transaction: %w", err)
	}
	return nil
}

type mfaWriter struct{ queries *generated.Queries }

func (w *mfaWriter) CreateChallenge(ctx context.Context, p mfa.CreateParams) error {
	if err := w.queries.CreateMfaChallenge(ctx, generated.CreateMfaChallengeParams{
		UserID: p.UserID, TokenHash: p.TokenHash, CodeHash: p.CodeHash,
		ExpiresAt: pgtype.Timestamptz{Time: p.ExpiresAt, Valid: true}, AttemptsLeft: int32(mfa.MaxAttempts),
	}); err != nil {
		return fmt.Errorf("create mfa challenge: %w", err)
	}
	return nil
}

func (w *mfaWriter) GetChallengeForUpdate(ctx context.Context, tokenHash []byte) (mfa.StoredChallenge, error) {
	row, err := w.queries.GetMfaChallengeForUpdate(ctx, tokenHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return mfa.StoredChallenge{}, mfa.ErrChallengeInvalid
	}
	if err != nil {
		return mfa.StoredChallenge{}, fmt.Errorf("get mfa challenge: %w", err)
	}
	return mfa.StoredChallenge{ID: row.ID, User: mfa.User{ID: row.UserID, Email: row.Email, DisplayName: row.DisplayName, Status: string(row.Status)}, CodeHash: row.CodeHash, ExpiresAt: row.ExpiresAt.Time, LastSentAt: row.LastSentAt.Time, Used: row.UsedAt.Valid, AttemptsLeft: int(row.AttemptsLeft)}, nil
}

func (w *mfaWriter) ConsumeChallenge(ctx context.Context, id uuid.UUID) error {
	if err := w.queries.ConsumeMfaChallenge(ctx, id); err != nil {
		return fmt.Errorf("consume mfa challenge: %w", err)
	}
	return nil
}
func (w *mfaWriter) RejectChallenge(ctx context.Context, id uuid.UUID) (int, error) {
	remaining, err := w.queries.RejectMfaChallenge(ctx, id)
	if err != nil {
		return 0, fmt.Errorf("reject mfa challenge: %w", err)
	}
	return int(remaining), nil
}
func (w *mfaWriter) ResendChallenge(ctx context.Context, id uuid.UUID, codeHash []byte) error {
	if err := w.queries.ResendMfaChallenge(ctx, generated.ResendMfaChallengeParams{ID: id, CodeHash: codeHash}); err != nil {
		return fmt.Errorf("resend mfa challenge: %w", err)
	}
	return nil
}
func (w *mfaWriter) ListRolesForUser(ctx context.Context, id uuid.UUID) ([]string, error) {
	roles, err := w.queries.ListRolesForUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list mfa roles: %w", err)
	}
	return roles, nil
}
func (w *mfaWriter) CreateRefreshToken(ctx context.Context, token mfa.RefreshToken) error {
	if err := w.queries.CreateRefreshToken(ctx, generated.CreateRefreshTokenParams{UserID: token.UserID, TokenHash: token.TokenHash, FamilyID: token.FamilyID, Ip: copyAddr(token.IP), UserAgent: nullableText(token.UserAgent), ExpiresAt: pgtype.Timestamptz{Time: token.ExpiresAt, Valid: true}}); err != nil {
		return fmt.Errorf("create mfa refresh token: %w", err)
	}
	return nil
}

func (w *mfaWriter) InsertAuditEvent(ctx context.Context, event mfa.AuditEvent) error {
	metadata := map[string]string{}
	if event.Reason != "" {
		metadata["reason"] = event.Reason
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal mfa audit metadata: %w", err)
	}
	if _, err := w.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{
		ActorUserID: nullableUUID(&event.ActorUserID), Action: event.Action, Ip: copyAddr(event.IP), UserAgent: nullableText(event.UserAgent), Metadata: encoded,
	}); err != nil {
		return fmt.Errorf("insert mfa audit event: %w", err)
	}
	return nil
}

func (w *mfaWriter) CountLoginFailuresByAccount(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	count, err := w.queries.CountLoginFailuresByAccount(ctx, generated.CountLoginFailuresByAccountParams{ActorUserID: nullableUUID(&userID), CreatedAt: pgtype.Timestamptz{Time: since, Valid: true}})
	if err != nil {
		return 0, fmt.Errorf("count mfa failures by account: %w", err)
	}
	return count, nil
}

func (w *mfaWriter) LockLoginUser(ctx context.Context, userID uuid.UUID, lockedUntil time.Time) error {
	if err := w.queries.LockLoginUser(ctx, generated.LockLoginUserParams{ID: userID, LockedUntil: pgtype.Timestamptz{Time: lockedUntil, Valid: true}}); err != nil {
		return fmt.Errorf("lock user after mfa failures: %w", err)
	}
	return nil
}

func (w *mfaWriter) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	if err := w.queries.UpdateLastLogin(ctx, userID); err != nil {
		return fmt.Errorf("update last login: %w", err)
	}
	return nil
}
