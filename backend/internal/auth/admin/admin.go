// Package admin implements the user-management rules reserved for administrators.
package admin

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrSelfDisable        = errors.New("an administrator cannot disable their own account")
	ErrSelfRoleAssignment = errors.New("an administrator cannot assign roles to their own account")
	ErrLastActiveAdmin    = errors.New("the system must retain an active administrator")
	ErrInvalidStatus      = errors.New("invalid user status")
	ErrInvalidRole        = errors.New("invalid user role")
	ErrBaseRoleRequired   = errors.New("every account must keep the base user role")
)

type Status string

const (
	StatusPendingVerification Status = "pending_verification"
	StatusActive              Status = "active"
	StatusLocked              Status = "locked"
	StatusDisabled            Status = "disabled"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	DisplayName  string
	Status       Status
	MFAEnabled   bool
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	Roles        []string
}

type ListInput struct {
	Query  string
	Status *Status
	Cursor *uuid.UUID
	Limit  int
}

type UpdateInput struct {
	ActorUserID uuid.UUID
	UserID      uuid.UUID
	Status      *Status
	Roles       *[]string
}

type AuditEvent struct {
	ActorUserID  *uuid.UUID
	Action       string
	ResourceType string
	ResourceID   string
}

type Writer interface {
	// LockActiveAdmins obtains row locks for every active administrator before
	// reading or changing a target, serializing concurrent demotions/disables.
	LockActiveAdmins(context.Context) (int64, error)
	GetUserForUpdate(context.Context, uuid.UUID) (User, error)
	ListRolesForUser(context.Context, uuid.UUID) ([]string, error)
	ValidateRoleNames(context.Context, []string) ([]string, error)
	UpdateUser(context.Context, uuid.UUID, Status) (User, error)
	ReplaceRoles(context.Context, uuid.UUID, []string, uuid.UUID) error
	InsertAuditEvent(context.Context, AuditEvent) error
}

type Repository interface {
	ListUsers(context.Context, ListInput) ([]User, error)
	GetUser(context.Context, uuid.UUID) (User, error)
	WithinUserManagementTransaction(context.Context, func(Writer) error) error
}

type Manager interface {
	ListUsers(context.Context, ListInput) ([]User, error)
	GetUser(context.Context, uuid.UUID) (User, error)
	UpdateUser(context.Context, UpdateInput) (User, error)
}

type Service struct{ repository Repository }

func New(repository Repository) *Service { return &Service{repository: repository} }

func (s *Service) ListUsers(ctx context.Context, input ListInput) ([]User, error) {
	if s.repository == nil {
		return nil, errors.New("admin user service is unavailable")
	}
	if input.Status != nil && !validStatus(*input.Status) {
		return nil, ErrInvalidStatus
	}
	if input.Limit < 1 {
		input.Limit = 1
	}
	// Handlers request one extra row to determine whether a 1-100 item page has
	// another cursor. The public page itself is still capped at 100 items.
	if input.Limit > 101 {
		input.Limit = 101
	}
	users, err := s.repository.ListUsers(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}

func (s *Service) GetUser(ctx context.Context, id uuid.UUID) (User, error) {
	if s.repository == nil {
		return User{}, errors.New("admin user service is unavailable")
	}
	user, err := s.repository.GetUser(ctx, id)
	if err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (s *Service) UpdateUser(ctx context.Context, input UpdateInput) (User, error) {
	if s.repository == nil {
		return User{}, errors.New("admin user service is unavailable")
	}
	// Invariant 11 (D8): an administrator never grants itself roles, even when
	// the requested set matches what it already has or is otherwise invalid.
	// It runs before request validation so that every self-assignment attempt
	// is audited; the audit needs its own committed transaction.
	if input.Roles != nil && input.ActorUserID == input.UserID {
		auditErr := s.repository.WithinUserManagementTransaction(ctx, func(writer Writer) error {
			return writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: &input.ActorUserID, Action: "role_assignment_rejected", ResourceType: "user", ResourceID: input.UserID.String()})
		})
		if auditErr != nil {
			return User{}, fmt.Errorf("audit rejected self role assignment: %w", auditErr)
		}
		return User{}, ErrSelfRoleAssignment
	}
	if input.Status != nil && !validStatus(*input.Status) {
		return User{}, ErrInvalidStatus
	}
	if input.Status != nil && *input.Status == StatusDisabled && input.ActorUserID == input.UserID {
		return User{}, ErrSelfDisable
	}

	var result User
	err := s.repository.WithinUserManagementTransaction(ctx, func(writer Writer) error {
		if input.Roles != nil {
			requested := deduplicate(*input.Roles)
			found, validateErr := writer.ValidateRoleNames(ctx, requested)
			if validateErr != nil {
				return fmt.Errorf("validate requested roles: %w", validateErr)
			}
			if len(found) != len(requested) {
				return ErrInvalidRole
			}
			// Roles is a full replacement, so user must remain present after
			// the database has established that every requested role exists.
			if !hasRole(requested, "user") {
				return ErrBaseRoleRequired
			}
		}
		activeAdmins, err := writer.LockActiveAdmins(ctx)
		if err != nil {
			return fmt.Errorf("lock active admins: %w", err)
		}
		target, err := writer.GetUserForUpdate(ctx, input.UserID)
		if err != nil {
			return fmt.Errorf("lock target user: %w", err)
		}
		currentRoles, err := writer.ListRolesForUser(ctx, input.UserID)
		if err != nil {
			return fmt.Errorf("list target roles: %w", err)
		}
		newStatus := target.Status
		if input.Status != nil {
			newStatus = *input.Status
		}
		newRoles := currentRoles
		if input.Roles != nil {
			newRoles = deduplicate(*input.Roles)
		}
		removesActiveAdmin := target.Status == StatusActive && hasRole(currentRoles, "admin") && (newStatus != StatusActive || !hasRole(newRoles, "admin"))
		if removesActiveAdmin && activeAdmins <= 1 {
			return ErrLastActiveAdmin
		}
		if input.Status != nil && newStatus != target.Status {
			target, err = writer.UpdateUser(ctx, input.UserID, newStatus)
			if err != nil {
				return fmt.Errorf("update user status: %w", err)
			}
			if newStatus == StatusDisabled {
				if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: &input.ActorUserID, Action: "user_disabled", ResourceType: "user", ResourceID: input.UserID.String()}); err != nil {
					return fmt.Errorf("audit user disabled: %w", err)
				}
			}
		}
		if input.Roles != nil && !sameRoles(currentRoles, newRoles) {
			if err := writer.ReplaceRoles(ctx, input.UserID, newRoles, input.ActorUserID); err != nil {
				return fmt.Errorf("replace user roles: %w", err)
			}
			if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: &input.ActorUserID, Action: "role_changed", ResourceType: "user", ResourceID: input.UserID.String()}); err != nil {
				return fmt.Errorf("audit role changed: %w", err)
			}
		}
		target.Roles = newRoles
		result = target
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return result, nil
}

func validStatus(status Status) bool {
	return status == StatusPendingVerification || status == StatusActive || status == StatusLocked || status == StatusDisabled
}
func hasRole(roles []string, role string) bool {
	for _, value := range roles {
		if value == role {
			return true
		}
	}
	return false
}
func deduplicate(roles []string) []string {
	result := make([]string, 0, len(roles))
	for _, role := range roles {
		if !hasRole(result, role) {
			result = append(result, role)
		}
	}
	return result
}
func sameRoles(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for _, role := range left {
		if !hasRole(right, role) {
			return false
		}
	}
	return true
}
