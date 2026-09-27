package store

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

// CreateInvitationTokenParams contains an invitation-purpose token hash
// (T5, RF-001/RF-002). It shares the verification_tokens table with
// CreateVerificationTokenParams but is always written with purpose
// 'invitation', never 'email_verification'.
type CreateInvitationTokenParams struct {
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
}

// EmployeeCreationWriter groups the writes that must succeed atomically when
// an administrator creates an employee account: the account itself, its
// directory/application role grants, its invitation token, and the audit
// trail (RF-001, RF-011).
type EmployeeCreationWriter interface {
	CreateUser(context.Context, CreateUserParams) (User, error)
	AddUserRole(context.Context, uuid.UUID, string, uuid.UUID) error
	CreateInvitationToken(context.Context, CreateInvitationTokenParams) error
	InsertAuditEvent(context.Context, InsertAuditEventParams) (AuditEvent, error)
}

// WithinEmployeeCreationTransaction runs every employee-creation write in one
// PostgreSQL transaction. A publisher-confirm failure returned by fn rolls it
// back, exactly like WithinRegistrationTransaction.
func (s *Store) WithinEmployeeCreationTransaction(ctx context.Context, fn func(EmployeeCreationWriter) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin employee creation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	writer := &employeeCreationWriter{queries: generated.New(tx)}
	if err := fn(writer); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit employee creation transaction: %w", err)
	}
	return nil
}

type employeeCreationWriter struct{ queries *generated.Queries }

func (w *employeeCreationWriter) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	user, err := w.queries.CreateUser(ctx, generated.CreateUserParams{Email: params.Email, PasswordHash: params.PasswordHash, DisplayName: params.DisplayName})
	if err != nil {
		return User{}, fmt.Errorf("create employee user: %w", err)
	}
	return userFromGenerated(user), nil
}

func (w *employeeCreationWriter) AddUserRole(ctx context.Context, userID uuid.UUID, roleName string, grantedBy uuid.UUID) error {
	if err := w.queries.AddUserRole(ctx, generated.AddUserRoleParams{UserID: userID, Name: roleName, GrantedBy: pgtype.UUID{Bytes: grantedBy, Valid: true}}); err != nil {
		return fmt.Errorf("add employee role: %w", err)
	}
	return nil
}

func (w *employeeCreationWriter) CreateInvitationToken(ctx context.Context, params CreateInvitationTokenParams) error {
	if err := w.queries.CreateInvitationToken(ctx, generated.CreateInvitationTokenParams{UserID: params.UserID, TokenHash: params.TokenHash, ExpiresAt: pgtype.Timestamptz{Time: params.ExpiresAt, Valid: true}}); err != nil {
		return fmt.Errorf("create invitation token: %w", err)
	}
	return nil
}

func (w *employeeCreationWriter) InsertAuditEvent(ctx context.Context, params InsertAuditEventParams) (AuditEvent, error) {
	event, err := w.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{ActorUserID: nullableUUID(params.ActorUserID), Action: params.Action, ResourceType: nullableText(params.ResourceType), ResourceID: nullableText(params.ResourceID), Ip: copyAddr(params.IP), UserAgent: nullableText(params.UserAgent), Metadata: params.Metadata})
	if err != nil {
		return AuditEvent{}, fmt.Errorf("insert audit event: %w", err)
	}
	return auditEventFromGenerated(event), nil
}
