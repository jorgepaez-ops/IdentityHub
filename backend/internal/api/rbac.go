package api

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/store"
)

type roleRepository interface {
	GetUserByID(context.Context, uuid.UUID) (store.User, error)
	ListRolesForUser(context.Context, uuid.UUID) ([]string, error)
}

// RequireRole authorizes a request from the current database state rather than
// the role claims that were present when its access token was issued.
func RequireRole(repository roleRepository, requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if deny := authorizeRole(r, repository, requiredRole); deny != nil {
				deny(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// authorizeRole checks the request in order and returns nil when it is allowed,
// or the function that writes the matching error response when it is not.
func authorizeRole(r *http.Request, repository roleRepository, requiredRole string) func(http.ResponseWriter) {
	userID, ok := currentUserID(r)
	if !ok {
		return writeUnauthorized
	}
	if repository == nil {
		return func(w http.ResponseWriter) {
			writeProblem(w, http.StatusServiceUnavailable, "authorization-unavailable", titleServiceUnavailable, "Authorization is temporarily unavailable.")
		}
	}
	user, err := repository.GetUserByID(r.Context(), userID)
	if err != nil {
		return writeAuthorizationLoadFailed
	}
	if user.Status != "active" {
		return writeUnauthorized
	}
	roles, err := repository.ListRolesForUser(r.Context(), userID)
	if err != nil {
		return writeAuthorizationLoadFailed
	}
	if hasRole(roles, requiredRole) {
		return nil
	}
	return func(w http.ResponseWriter) {
		writeProblem(w, http.StatusForbidden, "forbidden", "Forbidden", "You do not have permission to access this resource.")
	}
}

// writeAuthorizationLoadFailed writes the 500 problem used when the user or
// their roles could not be loaded.
func writeAuthorizationLoadFailed(w http.ResponseWriter) {
	writeProblem(w, http.StatusInternalServerError, "authorization-load-failed", titleInternalServerError, "Authorization could not be verified.")
}

// hasRole reports whether roles contains required.
func hasRole(roles []string, required string) bool {
	for _, role := range roles {
		if role == required {
			return true
		}
	}
	return false
}
