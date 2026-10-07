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
	if err := s.rejectSelfRoleAssignment(ctx, input); err != nil {
		return User{}, err
	}
	if err := validateStatusChange(input); err != nil {
		return User{}, err
	}

	var result User
	err := s.repository.WithinUserManagementTransaction(ctx, func(writer Writer) error {
		updated, err := applyUserUpdate(ctx, writer, input)
		if err != nil {
			return err
		}
		result = updated
		return nil
	})
	if err != nil {
		return User{}, err
	}
	return result, nil
}

// rejectSelfRoleAssignment returns ErrSelfRoleAssignment when the actor tries to
// set its own roles, after auditing the attempt in its own committed
// transaction. It returns nil when the request is not a self role assignment.
func (s *Service) rejectSelfRoleAssignment(ctx context.Context, input UpdateInput) error {
	// Invariant 11 (D8): an administrator never grants itself roles, even when
	// the requested set matches what it already has or is otherwise invalid.
	// It runs before request validation so that every self-assignment attempt
	// is audited; the audit needs its own committed transaction.
	if input.Roles == nil || input.ActorUserID != input.UserID {
		return nil
	}
	auditErr := s.repository.WithinUserManagementTransaction(ctx, func(writer Writer) error {
		return writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: &input.ActorUserID, Action: "role_assignment_rejected", ResourceType: "user", ResourceID: input.UserID.String()})
	})
	if auditErr != nil {
		return fmt.Errorf("audit rejected self role assignment: %w", auditErr)
	}
	return ErrSelfRoleAssignment
}

// validateStatusChange rejects unknown statuses and an actor disabling itself
// before any transaction is opened.
func validateStatusChange(input UpdateInput) error {
	if input.Status == nil {
		return nil
	}
	if !validStatus(*input.Status) {
		return ErrInvalidStatus
	}
	if *input.Status == StatusDisabled && input.ActorUserID == input.UserID {
		return ErrSelfDisable
	}
	return nil
}

// applyUserUpdate runs the whole update inside an open transaction and returns
// the target user with its resulting roles.
func applyUserUpdate(ctx context.Context, writer Writer, input UpdateInput) (User, error) {
	if err := validateRequestedRoles(ctx, writer, input); err != nil {
		return User{}, err
	}
	activeAdmins, target, currentRoles, err := lockAndLoadTarget(ctx, writer, input.UserID)
	if err != nil {
		return User{}, err
	}
	newStatus := target.Status
	if input.Status != nil {
		newStatus = *input.Status
	}
	newRoles := currentRoles
	if input.Roles != nil {
		newRoles = deduplicate(*input.Roles)
	}
	if removesActiveAdmin(target, currentRoles, newStatus, newRoles) && activeAdmins <= 1 {
		return User{}, ErrLastActiveAdmin
	}
	target, err = applyStatusChange(ctx, writer, input, target, newStatus)
	if err != nil {
		return User{}, err
	}
	if err := applyRoleChange(ctx, writer, input, currentRoles, newRoles); err != nil {
		return User{}, err
	}
	target.Roles = newRoles
	return target, nil
}

// validateRequestedRoles checks, when roles are requested, that every role
// exists and that the base user role is kept.
func validateRequestedRoles(ctx context.Context, writer Writer, input UpdateInput) error {
	if input.Roles == nil {
		return nil
	}
	requested := deduplicate(*input.Roles)
	found, err := writer.ValidateRoleNames(ctx, requested)
	if err != nil {
		return fmt.Errorf("validate requested roles: %w", err)
	}
	if len(found) != len(requested) {
		return ErrInvalidRole
	}
	// Roles is a full replacement, so user must remain present after
	// the database has established that every requested role exists.
	if !hasRole(requested, "user") {
		return ErrBaseRoleRequired
	}
	return nil
}

// lockAndLoadTarget locks the active administrators, then locks and loads the
// target user and its current roles, in that order.
func lockAndLoadTarget(ctx context.Context, writer Writer, userID uuid.UUID) (int64, User, []string, error) {
	activeAdmins, err := writer.LockActiveAdmins(ctx)
	if err != nil {
		return 0, User{}, nil, fmt.Errorf("lock active admins: %w", err)
	}
	target, err := writer.GetUserForUpdate(ctx, userID)
	if err != nil {
		return 0, User{}, nil, fmt.Errorf("lock target user: %w", err)
	}
	currentRoles, err := writer.ListRolesForUser(ctx, userID)
	if err != nil {
		return 0, User{}, nil, fmt.Errorf("list target roles: %w", err)
	}
	return activeAdmins, target, currentRoles, nil
}

// removesActiveAdmin reports whether the update would take an active
// administrator out of the active-admin set.
func removesActiveAdmin(target User, currentRoles []string, newStatus Status, newRoles []string) bool {
	return target.Status == StatusActive && hasRole(currentRoles, "admin") && (newStatus != StatusActive || !hasRole(newRoles, "admin"))
}

// applyStatusChange persists a status change when one was requested and differs
// from the current one, auditing a disable. It returns the updated target, or
// the unchanged target when nothing ran.
func applyStatusChange(ctx context.Context, writer Writer, input UpdateInput, target User, newStatus Status) (User, error) {
	if input.Status == nil || newStatus == target.Status {
		return target, nil
	}
	updated, err := writer.UpdateUser(ctx, input.UserID, newStatus)
	if err != nil {
		return User{}, fmt.Errorf("update user status: %w", err)
	}
	if newStatus == StatusDisabled {
		if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: &input.ActorUserID, Action: "user_disabled", ResourceType: "user", ResourceID: input.UserID.String()}); err != nil {
			return User{}, fmt.Errorf("audit user disabled: %w", err)
		}
	}
	return updated, nil
}

// applyRoleChange replaces the user's roles and audits it when roles were
// requested and differ from the current set.
func applyRoleChange(ctx context.Context, writer Writer, input UpdateInput, currentRoles, newRoles []string) error {
	if input.Roles == nil || sameRoles(currentRoles, newRoles) {
		return nil
	}
	if err := writer.ReplaceRoles(ctx, input.UserID, newRoles, input.ActorUserID); err != nil {
		return fmt.Errorf("replace user roles: %w", err)
	}
	if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: &input.ActorUserID, Action: "role_changed", ResourceType: "user", ResourceID: input.UserID.String()}); err != nil {
		return fmt.Errorf("audit role changed: %w", err)
	}
	return nil
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
