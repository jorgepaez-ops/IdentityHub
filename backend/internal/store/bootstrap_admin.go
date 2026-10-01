package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

// BootstrapAdminWriter contains the transaction-bound operations needed to
// decide and, only when needed, create the installation's first administrator.
type BootstrapAdminWriter interface {
	LockBootstrapAdmin(context.Context) error
	NonPendingAdminExists(context.Context) (bool, error)
	GetUserByEmail(context.Context, string) (User, error)
	CreateUser(context.Context, CreateUserParams) (User, error)
	AddBootstrapUserRole(context.Context, uuid.UUID, string) error
	ListRolesForUser(context.Context, uuid.UUID) ([]string, error)
	HasLiveInvitationToken(context.Context, uuid.UUID) (bool, error)
	InvalidateInvitationTokens(context.Context, uuid.UUID) error
	CreateInvitationToken(context.Context, CreateInvitationTokenParams) error
	InsertAuditEvent(context.Context, InsertAuditEventParams) (AuditEvent, error)
}

// WithinBootstrapAdminTransaction keeps the process-wide advisory lock and
// every bootstrap write on the same PostgreSQL transaction/connection.
func (s *Store) WithinBootstrapAdminTransaction(ctx context.Context, fn func(BootstrapAdminWriter) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin bootstrap administrator transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := fn(&bootstrapAdminWriter{queries: generated.New(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit bootstrap administrator transaction: %w", err)
	}
	return nil
}

type bootstrapAdminWriter struct{ queries *generated.Queries }

func (w *bootstrapAdminWriter) LockBootstrapAdmin(ctx context.Context) error {
	if err := w.queries.LockBootstrapAdmin(ctx); err != nil {
		return fmt.Errorf("lock bootstrap administrator: %w", err)
	}
	return nil
}

func (w *bootstrapAdminWriter) NonPendingAdminExists(ctx context.Context) (bool, error) {
	exists, err := w.queries.NonPendingAdminExists(ctx)
	if err != nil {
		return false, fmt.Errorf("check existing administrator: %w", err)
	}
	return exists, nil
}

func (w *bootstrapAdminWriter) GetUserByEmail(ctx context.Context, email string) (User, error) {
	user, err := w.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return User{}, fmt.Errorf("get bootstrap user by email: %w", err)
	}
	return userFromGenerated(user), nil
}

func (w *bootstrapAdminWriter) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	user, err := w.queries.CreateUser(ctx, generated.CreateUserParams{
		Email: params.Email, PasswordHash: params.PasswordHash, DisplayName: params.DisplayName,
	})
	if err != nil {
		return User{}, fmt.Errorf("create bootstrap user: %w", err)
	}
	return userFromGenerated(user), nil
}

func (w *bootstrapAdminWriter) AddBootstrapUserRole(ctx context.Context, userID uuid.UUID, roleName string) error {
	rows, err := w.queries.AddBootstrapUserRole(ctx, generated.AddBootstrapUserRoleParams{UserID: userID, Name: roleName})
	if err != nil {
		return fmt.Errorf("add bootstrap role: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("add bootstrap role: role %q is missing", roleName)
	}
	return nil
}

func (w *bootstrapAdminWriter) ListRolesForUser(ctx context.Context, userID uuid.UUID) ([]string, error) {
	names, err := w.queries.ListRolesForUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list bootstrap user roles: %w", err)
	}
	return names, nil
}

func (w *bootstrapAdminWriter) HasLiveInvitationToken(ctx context.Context, userID uuid.UUID) (bool, error) {
	live, err := w.queries.HasLiveInvitationToken(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("check live bootstrap invitation token: %w", err)
	}
	return live, nil
}

func (w *bootstrapAdminWriter) InvalidateInvitationTokens(ctx context.Context, userID uuid.UUID) error {
	if err := w.queries.InvalidateInvitationTokens(ctx, userID); err != nil {
		return fmt.Errorf("invalidate bootstrap invitation tokens: %w", err)
	}
	return nil
}

func (w *bootstrapAdminWriter) CreateInvitationToken(ctx context.Context, params CreateInvitationTokenParams) error {
	if err := w.queries.CreateInvitationToken(ctx, generated.CreateInvitationTokenParams{UserID: params.UserID, TokenHash: params.TokenHash, ExpiresAt: pgtype.Timestamptz{Time: params.ExpiresAt, Valid: true}}); err != nil {
		return fmt.Errorf("create bootstrap invitation token: %w", err)
	}
	return nil
}

func (w *bootstrapAdminWriter) InsertAuditEvent(ctx context.Context, params InsertAuditEventParams) (AuditEvent, error) {
	event, err := w.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{ActorUserID: nullableUUID(params.ActorUserID), Action: params.Action, ResourceType: nullableText(params.ResourceType), ResourceID: nullableText(params.ResourceID), Ip: copyAddr(params.IP), UserAgent: nullableText(params.UserAgent), Metadata: params.Metadata})
	if err != nil {
		return AuditEvent{}, fmt.Errorf("insert bootstrap audit event: %w", err)
	}
	return auditEventFromGenerated(event), nil
}
