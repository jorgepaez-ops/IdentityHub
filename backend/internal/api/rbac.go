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
			userID, ok := currentUserID(r)
			if !ok {
				writeUnauthorized(w)
				return
			}
			if repository == nil {
				writeProblem(w, http.StatusServiceUnavailable, "authorization-unavailable", "Service Unavailable", "Authorization is temporarily unavailable.")
				return
			}
			user, err := repository.GetUserByID(r.Context(), userID)
			if err != nil {
				writeProblem(w, http.StatusInternalServerError, "authorization-load-failed", "Internal Server Error", "Authorization could not be verified.")
				return
			}
			if user.Status != "active" {
				writeUnauthorized(w)
				return
			}
			roles, err := repository.ListRolesForUser(r.Context(), userID)
			if err != nil {
				writeProblem(w, http.StatusInternalServerError, "authorization-load-failed", "Internal Server Error", "Authorization could not be verified.")
				return
			}
			for _, role := range roles {
				if role == requiredRole {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeProblem(w, http.StatusForbidden, "forbidden", "Forbidden", "You do not have permission to access this resource.")
		})
	}
}
