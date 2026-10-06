package store

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jorgepaez/identity-hub/internal/auth/rolegrid"
)

func TestRF021_MapRoleGridErrorDistingueRestriccionesPorNombre(t *testing.T) {
	unknown := &pgconn.PgError{Code: "23505", ConstraintName: "some_other_key"}
	tests := []struct {
		name string
		err  error
		want error
		// passthrough marks errors that must stay reachable through errors.As after wrapping.
		passthrough bool
	}{
		{"nil", nil, nil, false},
		{"duplicate name", &pgconn.PgError{Code: "23505", ConstraintName: "roles_name_key"}, rolegrid.ErrDuplicateRole, false},
		{"assigned role", &pgconn.PgError{Code: "23503", ConstraintName: "user_roles_role_id_fkey"}, rolegrid.ErrRoleAssigned, false},
		{"missing application", &pgconn.PgError{Code: "23503", ConstraintName: "roles_application_id_fkey"}, rolegrid.ErrApplicationNotFound, false},
		{"system role trigger", &pgconn.PgError{Code: "P0001", Message: "system roles cannot be updated or deleted"}, rolegrid.ErrSystemRole, false},
		{"other unique violation", unknown, nil, true},
		{"other foreign key", &pgconn.PgError{Code: "23503", ConstraintName: "role_permissions_permission_id_fkey"}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapRoleGridError(tt.err)
			if tt.err == nil {
				if got != nil {
					t.Fatalf("got %v, want nil", got)
				}
				return
			}
			for _, typed := range []error{rolegrid.ErrDuplicateRole, rolegrid.ErrRoleAssigned, rolegrid.ErrApplicationNotFound, rolegrid.ErrSystemRole} {
				if errors.Is(got, typed) != errors.Is(tt.want, typed) {
					t.Fatalf("got %v, want %v", got, tt.want)
				}
			}
			var pgErr *pgconn.PgError
			if tt.passthrough && !errors.As(got, &pgErr) {
				t.Fatalf("an unmapped database error must stay reachable through %%w, got %v", got)
			}
		})
	}
}
