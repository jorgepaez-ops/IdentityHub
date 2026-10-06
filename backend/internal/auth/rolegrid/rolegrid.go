// Package rolegrid implements the administration of configurable application
// roles (RF-021, ADR 0013): every mutation enforces the five controls of the
// ADR and is audited in the same transaction as the change.
package rolegrid

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Audit actions written by this package. internal/audit aliases these as its
// Action constants (rolegrid cannot import audit without an import cycle).
const (
	ActionRoleCreated = "role_created"
	ActionRoleUpdated = "role_updated"
	ActionRoleDeleted = "role_deleted"
)

// MaxDescriptionLength bounds a role description, in characters.
const MaxDescriptionLength = 500

var (
	ErrApplicationNotFound  = errors.New("application not found")
	ErrRoleNotFound         = errors.New("application role not found")
	ErrInvalidRoleName      = errors.New("invalid application role name")
	ErrInvalidPermission    = errors.New("unknown permission for this application")
	ErrInvalidDescription   = errors.New("role description is too long")
	ErrEmptyUpdate          = errors.New("update must include description or permissions")
	ErrSelfPermissionChange = errors.New("an administrator cannot change permissions of a role they hold")
	ErrSelfRoleDelete       = errors.New("an administrator cannot delete a role they hold")
	ErrRoleAssigned         = errors.New("application role has assignments")
	ErrSystemRole           = errors.New("system roles cannot be changed")
	ErrDuplicateRole        = errors.New("application role name already exists")
)

// roleSlug is the part of a role name after "<client_id>.".
var roleSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{1,40}$`)

type Permission struct{ Key, Description string }

type Application struct {
	ID          uuid.UUID
	ClientID    string
	Name        string
	Permissions []Permission
	Roles       []Role
}

type Role struct {
	ID             uuid.UUID
	ApplicationID  uuid.UUID
	Name           string
	Description    string
	PermissionKeys []string
	System         bool
	AssignedCount  int64
}

type AuditEvent struct {
	ActorUserID    uuid.UUID
	Action         string
	RoleID         uuid.UUID
	RoleName       string
	ApplicationID  uuid.UUID
	PermissionKeys []string
	IP             netip.Addr
	UserAgent      string
}

type CreateInput struct {
	ActorUserID, ApplicationID uuid.UUID
	Name, Description          string
	PermissionKeys             []string
	IP                         netip.Addr
	UserAgent                  string
}

// UpdateInput changes only what is non-nil; the name is never renamed.
type UpdateInput struct {
	ActorUserID, ApplicationID, RoleID uuid.UUID
	Description                        *string
	PermissionKeys                     *[]string
	IP                                 netip.Addr
	UserAgent                          string
}

type DeleteInput struct {
	ActorUserID, ApplicationID, RoleID uuid.UUID
	IP                                 netip.Addr
	UserAgent                          string
}

// Writer groups the operations that must share one transaction.
type Writer interface {
	GetApplication(context.Context, uuid.UUID) (Application, error)
	// GetApplicationRoleForUpdate locks the role row (FOR UPDATE) until the transaction ends and
	// returns it with its permission keys and assignment count, so controls 2 and 4 are
	// evaluated against a role that no concurrent assignment or change can alter.
	GetApplicationRoleForUpdate(ctx context.Context, applicationID, roleID uuid.UUID) (Role, error)
	// ValidatePermissionKeys returns the subset of keys declared by the application.
	ValidatePermissionKeys(context.Context, uuid.UUID, []string) ([]string, error)
	ActorHoldsApplicationRole(ctx context.Context, userID, roleID uuid.UUID) (bool, error)
	CreateApplicationRole(ctx context.Context, applicationID uuid.UUID, name, description string) (Role, error)
	ReplaceApplicationRolePermissions(ctx context.Context, roleID, applicationID uuid.UUID, keys []string) error
	UpdateApplicationRoleDescription(ctx context.Context, roleID uuid.UUID, description string) error
	DeleteApplicationRole(context.Context, uuid.UUID) error
	InsertRoleGridAuditEvent(context.Context, AuditEvent) error
}

type Repository interface {
	ListApplications(context.Context) ([]Application, error)
	ListRoles(context.Context, uuid.UUID) ([]Role, error)
	WithinRoleGridTransaction(context.Context, func(Writer) error) error
}

type Manager interface {
	ListApplications(context.Context) ([]Application, error)
	ListRoles(context.Context, uuid.UUID) ([]Role, error)
	Create(context.Context, CreateInput) (Role, error)
	Update(context.Context, UpdateInput) (Role, error)
	Delete(context.Context, DeleteInput) error
}

type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

var errUnavailable = errors.New("role grid service is unavailable")

func (s *Service) ListApplications(ctx context.Context) ([]Application, error) {
	if s.repository == nil {
		return nil, errUnavailable
	}
	return s.repository.ListApplications(ctx)
}

func (s *Service) ListRoles(ctx context.Context, applicationID uuid.UUID) ([]Role, error) {
	if s.repository == nil {
		return nil, errUnavailable
	}
	return s.repository.ListRoles(ctx, applicationID)
}

func (s *Service) Create(ctx context.Context, input CreateInput) (Role, error) {
	if s.repository == nil {
		return Role{}, errUnavailable
	}
	var result Role
	err := s.repository.WithinRoleGridTransaction(ctx, func(w Writer) error {
		application, err := w.GetApplication(ctx, input.ApplicationID)
		if err != nil {
			return err
		}
		// Control 1: only "<client_id>.<slug>" names, so "admin"/"user" can never be created here.
		if !validName(application.ClientID, input.Name) {
			return ErrInvalidRoleName
		}
		if utf8.RuneCountInString(input.Description) > MaxDescriptionLength {
			return ErrInvalidDescription
		}
		keys, err := validatePermissions(ctx, w, input.ApplicationID, input.PermissionKeys)
		if err != nil {
			return err
		}
		role, err := w.CreateApplicationRole(ctx, input.ApplicationID, input.Name, input.Description)
		if err != nil {
			return fmt.Errorf("create role: %w", err)
		}
		if err := w.ReplaceApplicationRolePermissions(ctx, role.ID, input.ApplicationID, keys); err != nil {
			return fmt.Errorf("set role permissions: %w", err)
		}
		role.PermissionKeys = keys
		if err := w.InsertRoleGridAuditEvent(ctx, auditEvent(ActionRoleCreated, input.ActorUserID, role, input.IP, input.UserAgent)); err != nil {
			return fmt.Errorf("audit role creation: %w", err)
		}
		result = role
		return nil
	})
	return result, err
}

func (s *Service) Update(ctx context.Context, input UpdateInput) (Role, error) {
	if s.repository == nil {
		return Role{}, errUnavailable
	}
	if input.Description == nil && input.PermissionKeys == nil {
		return Role{}, ErrEmptyUpdate
	}
	var result Role
	err := s.repository.WithinRoleGridTransaction(ctx, func(w Writer) error {
		role, err := w.GetApplicationRoleForUpdate(ctx, input.ApplicationID, input.RoleID)
		if err != nil {
			return err
		}
		if role.System {
			return ErrSystemRole
		}
		if input.Description != nil && utf8.RuneCountInString(*input.Description) > MaxDescriptionLength {
			return ErrInvalidDescription
		}
		if input.PermissionKeys != nil {
			// Control 2: nobody widens (or narrows) the privileges of a role they hold themselves.
			held, err := w.ActorHoldsApplicationRole(ctx, input.ActorUserID, role.ID)
			if err != nil {
				return fmt.Errorf("check actor role: %w", err)
			}
			if held {
				return ErrSelfPermissionChange
			}
			keys, err := validatePermissions(ctx, w, input.ApplicationID, *input.PermissionKeys)
			if err != nil {
				return err
			}
			if err := w.ReplaceApplicationRolePermissions(ctx, role.ID, input.ApplicationID, keys); err != nil {
				return fmt.Errorf("set role permissions: %w", err)
			}
			role.PermissionKeys = keys
		}
		if input.Description != nil {
			if err := w.UpdateApplicationRoleDescription(ctx, role.ID, *input.Description); err != nil {
				return fmt.Errorf("update role description: %w", err)
			}
			role.Description = *input.Description
		}
		if err := w.InsertRoleGridAuditEvent(ctx, auditEvent(ActionRoleUpdated, input.ActorUserID, role, input.IP, input.UserAgent)); err != nil {
			return fmt.Errorf("audit role update: %w", err)
		}
		result = role
		return nil
	})
	return result, err
}

func (s *Service) Delete(ctx context.Context, input DeleteInput) error {
	if s.repository == nil {
		return errUnavailable
	}
	return s.repository.WithinRoleGridTransaction(ctx, func(w Writer) error {
		role, err := w.GetApplicationRoleForUpdate(ctx, input.ApplicationID, input.RoleID)
		if err != nil {
			return err
		}
		if role.System {
			return ErrSystemRole
		}
		// Control 2 runs before control 4: an administrator who holds the role gets the
		// specific self-lockout error (403) instead of the generic assignment conflict (409),
		// which would also be true because they hold it.
		held, err := w.ActorHoldsApplicationRole(ctx, input.ActorUserID, role.ID)
		if err != nil {
			return fmt.Errorf("check actor role: %w", err)
		}
		if held {
			return ErrSelfRoleDelete
		}
		// Control 4: an assigned role is never deleted.
		if role.AssignedCount > 0 {
			return ErrRoleAssigned
		}
		if err := w.DeleteApplicationRole(ctx, role.ID); err != nil {
			return fmt.Errorf("delete role: %w", err)
		}
		if err := w.InsertRoleGridAuditEvent(ctx, auditEvent(ActionRoleDeleted, input.ActorUserID, role, input.IP, input.UserAgent)); err != nil {
			return fmt.Errorf("audit role deletion: %w", err)
		}
		return nil
	})
}

func validName(clientID, name string) bool {
	slug, ok := strings.CutPrefix(name, clientID+".")
	return ok && roleSlug.MatchString(slug)
}

// validatePermissions implements control 3 and returns the keys without
// duplicates, sorted. Unknown or other-application keys are ErrInvalidPermission.
func validatePermissions(ctx context.Context, w Writer, applicationID uuid.UUID, keys []string) ([]string, error) {
	unique := make([]string, 0, len(keys))
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		if !seen[key] {
			seen[key] = true
			unique = append(unique, key)
		}
	}
	sort.Strings(unique)
	found, err := w.ValidatePermissionKeys(ctx, applicationID, unique)
	if err != nil {
		return nil, fmt.Errorf("validate permissions: %w", err)
	}
	if len(found) != len(unique) {
		return nil, ErrInvalidPermission
	}
	return unique, nil
}

func auditEvent(action string, actor uuid.UUID, role Role, ip netip.Addr, userAgent string) AuditEvent {
	return AuditEvent{ActorUserID: actor, Action: action, RoleID: role.ID, RoleName: role.Name, ApplicationID: role.ApplicationID, PermissionKeys: append([]string{}, role.PermissionKeys...), IP: ip, UserAgent: userAgent}
}
