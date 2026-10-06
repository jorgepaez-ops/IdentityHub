package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jorgepaez/identity-hub/internal/auth/rolegrid"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

// ListApplications returns every application with its declared permissions and
// its configurable roles (permission keys and assignment counts included).
func (s *Store) ListApplications(ctx context.Context) ([]rolegrid.Application, error) {
	apps, err := s.queries.ListRoleGridApplications(ctx)
	if err != nil {
		return nil, fmt.Errorf("list applications: %w", err)
	}
	result := make([]rolegrid.Application, 0, len(apps))
	for _, app := range apps {
		permissions, err := s.queries.ListApplicationPermissions(ctx, app.ID)
		if err != nil {
			return nil, fmt.Errorf("list application permissions: %w", err)
		}
		roles, err := s.ListRoles(ctx, app.ID)
		if err != nil {
			return nil, err
		}
		item := rolegrid.Application{ID: app.ID, ClientID: app.ClientID, Name: app.Name, Permissions: make([]rolegrid.Permission, 0, len(permissions)), Roles: roles}
		for _, permission := range permissions {
			item.Permissions = append(item.Permissions, rolegrid.Permission{Key: permission.Key, Description: permission.Description})
		}
		result = append(result, item)
	}
	return result, nil
}

// ListRoles returns the configurable roles of one application, or ErrApplicationNotFound.
func (s *Store) ListRoles(ctx context.Context, applicationID uuid.UUID) ([]rolegrid.Role, error) {
	if _, err := s.queries.GetRoleGridApplication(ctx, applicationID); errors.Is(err, pgx.ErrNoRows) {
		return nil, rolegrid.ErrApplicationNotFound
	} else if err != nil {
		return nil, fmt.Errorf("get application: %w", err)
	}
	rows, err := s.queries.ListApplicationRoles(ctx, pgtype.UUID{Bytes: applicationID, Valid: true})
	if err != nil {
		return nil, fmt.Errorf("list application roles: %w", err)
	}
	roles := make([]rolegrid.Role, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, rolegrid.Role{ID: row.ID, ApplicationID: row.ApplicationID.Bytes, Name: row.Name, Description: row.Description, PermissionKeys: row.PermissionKeys, System: row.System, AssignedCount: row.AssignedCount})
	}
	return roles, nil
}

// WithinRoleGridTransaction keeps validation, the role change and its audit
// event in one PostgreSQL transaction.
func (s *Store) WithinRoleGridTransaction(ctx context.Context, fn func(rolegrid.Writer) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin role grid transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(&roleGridWriter{queries: generated.New(tx)}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit role grid transaction: %w", err)
	}
	return nil
}

type roleGridWriter struct{ queries *generated.Queries }

func (w *roleGridWriter) GetApplication(ctx context.Context, id uuid.UUID) (rolegrid.Application, error) {
	row, err := w.queries.GetRoleGridApplication(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return rolegrid.Application{}, rolegrid.ErrApplicationNotFound
	}
	if err != nil {
		return rolegrid.Application{}, fmt.Errorf("get application: %w", err)
	}
	return rolegrid.Application{ID: row.ID, ClientID: row.ClientID, Name: row.Name}, nil
}

func (w *roleGridWriter) GetApplicationRoleForUpdate(ctx context.Context, applicationID, roleID uuid.UUID) (rolegrid.Role, error) {
	// The lock comes first: the read below then runs after any concurrent writer committed
	// (READ COMMITTED), and assigning the role (user_roles FK) waits for this row lock.
	_, err := w.queries.LockApplicationRole(ctx, generated.LockApplicationRoleParams{ID: roleID, ApplicationID: pgtype.UUID{Bytes: applicationID, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return rolegrid.Role{}, rolegrid.ErrRoleNotFound
	}
	if err != nil {
		return rolegrid.Role{}, fmt.Errorf("lock application role: %w", err)
	}
	row, err := w.queries.GetApplicationRole(ctx, generated.GetApplicationRoleParams{ID: roleID, ApplicationID: pgtype.UUID{Bytes: applicationID, Valid: true}})
	if errors.Is(err, pgx.ErrNoRows) {
		return rolegrid.Role{}, rolegrid.ErrRoleNotFound
	}
	if err != nil {
		return rolegrid.Role{}, fmt.Errorf("get application role: %w", err)
	}
	return rolegrid.Role{ID: row.ID, ApplicationID: row.ApplicationID.Bytes, Name: row.Name, Description: row.Description, PermissionKeys: row.PermissionKeys, System: row.System, AssignedCount: row.AssignedCount}, nil
}

func (w *roleGridWriter) ValidatePermissionKeys(ctx context.Context, applicationID uuid.UUID, keys []string) ([]string, error) {
	return w.queries.ValidateApplicationPermissionKeys(ctx, generated.ValidateApplicationPermissionKeysParams{ApplicationID: applicationID, Keys: keys})
}

func (w *roleGridWriter) ActorHoldsApplicationRole(ctx context.Context, userID, roleID uuid.UUID) (bool, error) {
	return w.queries.ActorHoldsApplicationRole(ctx, generated.ActorHoldsApplicationRoleParams{UserID: userID, RoleID: roleID})
}

func (w *roleGridWriter) CreateApplicationRole(ctx context.Context, applicationID uuid.UUID, name, description string) (rolegrid.Role, error) {
	row, err := w.queries.CreateApplicationRole(ctx, generated.CreateApplicationRoleParams{ApplicationID: pgtype.UUID{Bytes: applicationID, Valid: true}, Name: name, Description: description})
	if err != nil {
		return rolegrid.Role{}, mapRoleGridError(err)
	}
	return rolegrid.Role{ID: row.ID, ApplicationID: applicationID, Name: row.Name, Description: row.Description, System: row.System}, nil
}

func (w *roleGridWriter) ReplaceApplicationRolePermissions(ctx context.Context, roleID, applicationID uuid.UUID, keys []string) error {
	if err := w.queries.DeleteApplicationRolePermissions(ctx, roleID); err != nil {
		return err
	}
	for _, key := range keys {
		rows, err := w.queries.AddApplicationRolePermission(ctx, generated.AddApplicationRolePermissionParams{RoleID: roleID, ApplicationID: applicationID, Key: key})
		if err != nil {
			return err
		}
		if rows != 1 {
			return rolegrid.ErrInvalidPermission
		}
	}
	return nil
}

func (w *roleGridWriter) UpdateApplicationRoleDescription(ctx context.Context, roleID uuid.UUID, description string) error {
	return mapRoleGridError(w.queries.UpdateApplicationRoleDescription(ctx, generated.UpdateApplicationRoleDescriptionParams{ID: roleID, Description: description}))
}

func (w *roleGridWriter) DeleteApplicationRole(ctx context.Context, roleID uuid.UUID) error {
	return mapRoleGridError(w.queries.DeleteApplicationRole(ctx, roleID))
}

func (w *roleGridWriter) InsertRoleGridAuditEvent(ctx context.Context, event rolegrid.AuditEvent) error {
	metadata, err := json.Marshal(map[string]any{"roleName": event.RoleName, "applicationId": event.ApplicationID.String(), "permissionKeys": event.PermissionKeys})
	if err != nil {
		return fmt.Errorf("marshal role audit metadata: %w", err)
	}
	resourceType, resourceID := "application_role", event.RoleID.String()
	params := generated.InsertAuditEventParams{ActorUserID: nullableUUID(&event.ActorUserID), Action: event.Action, ResourceType: nullableText(&resourceType), ResourceID: nullableText(&resourceID), Metadata: metadata}
	if event.UserAgent != "" {
		params.UserAgent = nullableText(&event.UserAgent)
	}
	if event.IP.IsValid() {
		params.Ip = &event.IP
	}
	if _, err := w.queries.InsertAuditEvent(ctx, params); err != nil {
		return fmt.Errorf("insert role audit event: %w", err)
	}
	return nil
}

// mapRoleGridError translates database refusals into typed errors by constraint
// name (roles_name_key, user_roles_role_id_fkey and roles_application_id_fkey, see
// migrations 000001 and 000009) and by the system-role trigger (defensive: the
// service already refuses system roles). Any other database error is wrapped.
func mapRoleGridError(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch {
	case pgErr.Code == "23505" && pgErr.ConstraintName == "roles_name_key":
		return rolegrid.ErrDuplicateRole
	case pgErr.Code == "23503" && pgErr.ConstraintName == "user_roles_role_id_fkey":
		return rolegrid.ErrRoleAssigned
	case pgErr.Code == "23503" && pgErr.ConstraintName == "roles_application_id_fkey":
		return rolegrid.ErrApplicationNotFound
	case pgErr.Code == "P0001" && strings.Contains(pgErr.Message, "system roles"):
		return rolegrid.ErrSystemRole
	}
	return fmt.Errorf("role grid database error (%s, constraint %q): %w", pgErr.Code, pgErr.ConstraintName, err)
}
