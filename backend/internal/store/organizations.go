package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jorgepaez/identity-hub/internal/auth/admin"
	generated "github.com/jorgepaez/identity-hub/internal/store/internal/sqlc"
)

// Fixed identifiers of the organizations seeded by migration 000011 (RF-022, ADR 0015).
var (
	// HubOrganizationID is the platform operator, home of the global administrators.
	HubOrganizationID = uuid.MustParse("00000000-0000-4000-8000-0000000000a1")
	// ContabilidadOrganizationID is "Contabilidad S.A.", owner of the Contabilidad application.
	ContabilidadOrganizationID = uuid.MustParse("00000000-0000-4000-8000-0000000000a2")
	// EmpresaBOrganizationID is "Empresa B", owner of the demo warehouse application (second company demo).
	EmpresaBOrganizationID = uuid.MustParse("00000000-0000-4000-8000-0000000000a3")
)

var (
	// ErrCrossOrganizationRole reports that a role of an application owned by another organization was
	// assigned to a user, or that a user's organization changed while they still hold such roles
	// (invariants 12-14). It maps SQLSTATE IH001 raised by the migration 000011 triggers.
	ErrCrossOrganizationRole = admin.ErrCrossOrganizationRole
	// ErrOrganizationRequired reports a user creation without an organization.
	ErrOrganizationRequired = errors.New("user organization is required")
)

// crossOrganizationSQLState is the custom SQLSTATE the organization triggers raise.
const crossOrganizationSQLState = "IH001"

// mapOrganizationError turns the trigger violation into ErrCrossOrganizationRole and leaves any other
// error untouched.
func mapOrganizationError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == crossOrganizationSQLState {
		return ErrCrossOrganizationRole
	}
	return err
}

// Organization is a company served by the Hub.
type Organization struct {
	ID        uuid.UUID
	Slug      string
	Name      string
	CreatedAt time.Time
}

// OrganizationApplication is an application owned by an organization.
type OrganizationApplication struct {
	ID             uuid.UUID
	ClientID       string
	Name           string
	OrganizationID uuid.UUID
}

// ListOrganizations returns every organization ordered by name.
func (s *Store) ListOrganizations(ctx context.Context) ([]Organization, error) {
	rows, err := s.queries.ListOrganizations(ctx)
	if err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	result := make([]Organization, 0, len(rows))
	for _, row := range rows {
		result = append(result, Organization{ID: row.ID, Slug: row.Slug, Name: row.Name, CreatedAt: row.CreatedAt.Time})
	}
	return result, nil
}

// GetUserOrganization returns the organization a user belongs to. The error preserves pgx.ErrNoRows
// for an unknown user.
func (s *Store) GetUserOrganization(ctx context.Context, userID uuid.UUID) (Organization, error) {
	row, err := s.queries.GetUserOrganization(ctx, userID)
	if err != nil {
		return Organization{}, fmt.Errorf("get user organization: %w", err)
	}
	return Organization{ID: row.ID, Slug: row.Slug, Name: row.Name, CreatedAt: row.CreatedAt.Time}, nil
}

// UserApplication is an application the user can launch: it belongs to the user's organization and the
// user holds at least one of its roles.
type UserApplication struct {
	ClientID  string
	Name      string
	LaunchURL string
}

// ListUserApplications returns the applications of the user's organization in which the user holds at
// least one role. A user without application roles gets an empty list.
func (s *Store) ListUserApplications(ctx context.Context, userID uuid.UUID) ([]UserApplication, error) {
	rows, err := s.queries.ListUserApplications(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list user applications: %w", err)
	}
	result := make([]UserApplication, 0, len(rows))
	for _, row := range rows {
		result = append(result, UserApplication{ClientID: row.ClientID, Name: row.Name, LaunchURL: row.AllowedOrigin})
	}
	return result, nil
}

// ListOrganizationApplications returns only the applications owned by the organization.
func (s *Store) ListOrganizationApplications(ctx context.Context, organizationID uuid.UUID) ([]OrganizationApplication, error) {
	rows, err := s.queries.ListOrganizationApplications(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("list organization applications: %w", err)
	}
	result := make([]OrganizationApplication, 0, len(rows))
	for _, row := range rows {
		result = append(result, OrganizationApplication{ID: row.ID, ClientID: row.ClientID, Name: row.Name, OrganizationID: row.OrganizationID})
	}
	return result, nil
}

// ListOrganizationUsers is ListUsers restricted to one organization, with the same filters and paging.
func (s *Store) ListOrganizationUsers(ctx context.Context, organizationID uuid.UUID, input admin.ListInput) ([]admin.User, error) {
	params := generated.ListOrganizationUsersParams{OrganizationID: organizationID, Query: input.Query, LimitCount: int64(input.Limit)}
	if input.Status != nil {
		params.Status = generated.NullUserStatus{UserStatus: generated.UserStatus(*input.Status), Valid: true}
	}
	if input.Cursor != nil {
		params.Cursor = pgtype.UUID{Bytes: *input.Cursor, Valid: true}
	}
	rows, err := s.queries.ListOrganizationUsers(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list organization users: %w", err)
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
