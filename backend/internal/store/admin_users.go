package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jorgepaez/identity-hub/internal/auth/admin"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

// ListUsers is the read adapter for the administrator user directory. sqlc
// binds the search text rather than interpolating it into the LIKE expression.
func (s *Store) ListUsers(ctx context.Context, input admin.ListInput) ([]admin.User, error) {
	params := generated.ListAdminUsersParams{Query: input.Query, LimitCount: int64(input.Limit)}
	if input.Status != nil {
		params.Status = generated.NullUserStatus{UserStatus: generated.UserStatus(*input.Status), Valid: true}
	}
	if input.Cursor != nil {
		params.Cursor = pgtype.UUID{Bytes: *input.Cursor, Valid: true}
	}
	rows, err := s.queries.ListAdminUsers(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list admin users: %w", err)
	}
	result := make([]admin.User, 0, len(rows))
	for _, row := range rows {
		roles, err := s.ListRolesForUser(ctx, row.ID)
		if err != nil {
			return nil, fmt.Errorf("list user roles: %w", err)
		}
		result = append(result, adminUserFromGenerated(row, roles))
	}
	return result, nil
}

func (s *Store) GetUser(ctx context.Context, id uuid.UUID) (admin.User, error) {
	user, err := s.queries.GetUserByID(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return admin.User{}, admin.ErrUserNotFound
	}
	if err != nil {
		return admin.User{}, fmt.Errorf("get admin user: %w", err)
	}
	roles, err := s.ListRolesForUser(ctx, id)
	if err != nil {
		return admin.User{}, fmt.Errorf("list user roles: %w", err)
	}
	return adminUserFromGenerated(user, roles), nil
}

// WithinUserManagementTransaction keeps the active-admin lock, target update,
// role changes, and audit events in one PostgreSQL transaction.
func (s *Store) WithinUserManagementTransaction(ctx context.Context, fn func(admin.Writer) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin user management transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(&adminWriter{queries: generated.New(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit user management transaction: %w", err)
	}
	return nil
}

type adminWriter struct{ queries *generated.Queries }

func (w *adminWriter) LockActiveAdmins(ctx context.Context) (int64, error) {
	ids, err := w.queries.LockActiveAdminUsers(ctx)
	if err != nil {
		return 0, fmt.Errorf("lock active admin users: %w", err)
	}
	return int64(len(ids)), nil
}
func (w *adminWriter) GetUserForUpdate(ctx context.Context, id uuid.UUID) (admin.User, error) {
	user, err := w.queries.GetUserByIDForUpdate(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return admin.User{}, admin.ErrUserNotFound
	}
	if err != nil {
		return admin.User{}, fmt.Errorf("get user for update: %w", err)
	}
	roles, err := w.queries.ListRolesForUser(ctx, id)
	if err != nil {
		return admin.User{}, fmt.Errorf("list target roles: %w", err)
	}
	return adminUserFromGenerated(user, roles), nil
}
func (w *adminWriter) ListRolesForUser(ctx context.Context, id uuid.UUID) ([]string, error) {
	roles, err := w.queries.ListRolesForUser(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list roles: %w", err)
	}
	return roles, nil
}
func (w *adminWriter) UpdateUser(ctx context.Context, id uuid.UUID, status admin.Status) (admin.User, error) {
	user, err := w.queries.UpdateAdminUserStatus(ctx, generated.UpdateAdminUserStatusParams{ID: id, Status: generated.UserStatus(status)})
	if err != nil {
		return admin.User{}, fmt.Errorf("update user: %w", err)
	}
	roles, err := w.queries.ListRolesForUser(ctx, id)
	if err != nil {
		return admin.User{}, fmt.Errorf("list updated roles: %w", err)
	}
	return adminUserFromGenerated(user, roles), nil
}
func (w *adminWriter) ReplaceRoles(ctx context.Context, id uuid.UUID, roles []string, grantedBy uuid.UUID) error {
	if err := w.queries.DeleteUserRoles(ctx, id); err != nil {
		return fmt.Errorf("delete user roles: %w", err)
	}
	for _, role := range roles {
		if err := w.queries.AddUserRole(ctx, generated.AddUserRoleParams{UserID: id, Name: role, GrantedBy: pgtype.UUID{Bytes: grantedBy, Valid: true}}); err != nil {
			return fmt.Errorf("add user role: %w", err)
		}
	}
	return nil
}
func (w *adminWriter) InsertAuditEvent(ctx context.Context, event admin.AuditEvent) error {
	metadata, err := json.Marshal(map[string]string{})
	if err != nil {
		return fmt.Errorf("marshal admin audit metadata: %w", err)
	}
	if _, err := w.queries.InsertAuditEvent(ctx, generated.InsertAuditEventParams{ActorUserID: nullableUUID(event.ActorUserID), Action: event.Action, ResourceType: nullableText(&event.ResourceType), ResourceID: nullableText(&event.ResourceID), Metadata: metadata}); err != nil {
		return fmt.Errorf("insert admin audit event: %w", err)
	}
	return nil
}

func adminUserFromGenerated(user generated.User, roles []string) admin.User {
	return admin.User{ID: user.ID, Email: user.Email, PasswordHash: user.PasswordHash, DisplayName: user.DisplayName, Status: admin.Status(user.Status), MFAEnabled: user.MfaEnabled, LastLoginAt: optionalTime(user.LastLoginAt), CreatedAt: user.CreatedAt.Time, Roles: roles}
}
