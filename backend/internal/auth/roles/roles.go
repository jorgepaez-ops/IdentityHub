// Package roles is the single source of truth for the role catalog fixed by
// D8 (odd/tasks/idp-semana-3.md) and RF-009: directory roles govern the
// Identity Hub itself, application roles govern one client application and
// share its "<application>." name prefix.
package roles

import "strings"

// Directory roles. Every account holds User; Admin manages the directory
// (RF-001, RF-010) and never grants itself privileges (invariant 11).
const (
	Admin = "admin"
	User  = "user"
)

// Contabilidad application roles (D8). ApplicationContabilidad is the shared
// name prefix before the dot.
const (
	ApplicationContabilidad = "contabilidad"

	ContabilidadSenior   = ApplicationContabilidad + ".senior"
	ContabilidadAnalista = ApplicationContabilidad + ".analista"
)

// Catalog lists every role name the system recognizes. A role outside this
// list is rejected wherever roles are validated or assigned (admin.validRoles).
var Catalog = []string{Admin, User, ContabilidadSenior, ContabilidadAnalista}

// Valid reports whether role belongs to Catalog.
func Valid(role string) bool {
	for _, candidate := range Catalog {
		if candidate == role {
			return true
		}
	}
	return false
}

// IsDirectory reports whether role is a Hub directory role (admin/user): the
// roles with no "<application>." prefix, valid for every account (D8, RF-009).
func IsDirectory(role string) bool {
	return role == Admin || role == User
}

// Application returns the application prefix of an application role (for
// example ApplicationContabilidad for ContabilidadSenior), or "" for a
// directory role or an unrecognized name.
func Application(role string) string {
	if IsDirectory(role) {
		return ""
	}
	if index := strings.IndexByte(role, '.'); index >= 0 {
		return role[:index]
	}
	return ""
}

// Directory filters allRoles down to the Hub directory roles (admin/user).
// The Identity Hub console access token carries only these (D8, RF-009): an
// application role must never reach the console's roles claim.
func Directory(allRoles []string) []string {
	filtered := make([]string, 0, len(allRoles))
	for _, role := range allRoles {
		if IsDirectory(role) {
			filtered = append(filtered, role)
		}
	}
	return filtered
}

// ForApplication filters allRoles down to the roles that belong to
// application (for example ApplicationContabilidad), for the
// application-scoped access token of T9. Use Directory for directory roles:
// it accepts only admin and user, never an unknown dotless name.
func ForApplication(allRoles []string, application string) []string {
	filtered := make([]string, 0, len(allRoles))
	for _, role := range allRoles {
		if Application(role) == application {
			filtered = append(filtered, role)
		}
	}
	return filtered
}
