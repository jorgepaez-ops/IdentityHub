package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

// CreatePasswordResetTokenParams holds a hash-only reset token with its expiry.
type CreatePasswordResetTokenParams struct {
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
}

// ConsumePasswordResetTokenParams contains the validated token hash and its new Argon2id password hash.
type ConsumePasswordResetTokenParams struct {
	TokenHash    []byte
	PasswordHash string
}

type PasswordResetRequestWriter interface {
	CreatePasswordResetTokenForEmail(context.Context, string, CreatePasswordResetTokenParams) (bool, error)
}

type PasswordResetConfirmationWriter interface {
	ConsumePasswordResetTokenAndRevokeSessions(context.Context, ConsumePasswordResetTokenParams) (User, error)
}

func (s *Store) PasswordResetTokenIsUsable(ctx context.Context, tokenHash []byte) (bool, error) {
	usable, err := s.queries.PasswordResetTokenIsUsable(ctx, tokenHash)
	if err != nil {
		return false, fmt.Errorf("check password reset token: %w", err)
	}
	return usable, nil
}

func (s *Store) WithinPasswordResetRequestTransaction(ctx context.Context, fn func(PasswordResetRequestWriter) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin password reset request transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(&passwordResetRequestWriter{queries: generated.New(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password reset request transaction: %w", err)
	}
	return nil
}

func (s *Store) WithinPasswordResetConfirmationTransaction(ctx context.Context, fn func(PasswordResetConfirmationWriter) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin password reset confirmation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(&passwordResetConfirmationWriter{queries: generated.New(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit password reset confirmation transaction: %w", err)
	}
	return nil
}

type passwordResetRequestWriter struct{ queries *generated.Queries }

func (w *passwordResetRequestWriter) CreatePasswordResetTokenForEmail(ctx context.Context, email string, params CreatePasswordResetTokenParams) (bool, error) {
	created, err := w.queries.CreatePasswordResetTokenForEmail(ctx, generated.CreatePasswordResetTokenForEmailParams{Email: email, TokenHash: params.TokenHash, ExpiresAt: pgtype.Timestamptz{Time: params.ExpiresAt, Valid: true}})
	if err != nil {
		return false, fmt.Errorf("create password reset token for email: %w", err)
	}
	return created, nil
}

type passwordResetConfirmationWriter struct{ queries *generated.Queries }

func (w *passwordResetConfirmationWriter) ConsumePasswordResetTokenAndRevokeSessions(ctx context.Context, params ConsumePasswordResetTokenParams) (User, error) {
	user, err := w.queries.ConsumePasswordResetTokenAndRevokeSessions(ctx, generated.ConsumePasswordResetTokenAndRevokeSessionsParams{TokenHash: params.TokenHash, PasswordHash: params.PasswordHash})
	if err != nil {
		return User{}, fmt.Errorf("consume password reset token and revoke sessions: %w", err)
	}
	return User{ID: user.ID, Email: user.Email, PasswordHash: user.PasswordHash, DisplayName: user.DisplayName, Status: string(user.Status), MFAEnabled: user.MfaEnabled, LastLoginAt: optionalTime(user.LastLoginAt), CreatedAt: user.CreatedAt.Time}, nil
}
