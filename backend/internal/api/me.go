package api

import (
	"context"
	"encoding/json"
	"net/http"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/store"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

type currentUserRepository interface {
	GetUserByID(context.Context, uuid.UUID) (store.User, error)
	ListRolesForUser(context.Context, uuid.UUID) ([]string, error)
	UpdateDisplayName(context.Context, uuid.UUID, string) (store.User, error)
}

// GetCurrentUser requires a validated access token before loading the current
// profile. The user identifier is derived exclusively from its subject claim.
func (s *Server) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	RequireAuth(s.tokens)(http.HandlerFunc(s.getCurrentUser)).ServeHTTP(w, r)
}

func (s *Server) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	user, roles, ok := s.loadCurrentUser(r.Context(), userID, w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, apiUser(user, roles))
}

// UpdateCurrentUser requires a validated access token before changing the
// current profile. Only displayName is accepted by the OpenAPI request type.
func (s *Server) UpdateCurrentUser(w http.ResponseWriter, r *http.Request) {
	RequireAuth(s.tokens)(http.HandlerFunc(s.updateCurrentUser)).ServeHTTP(w, r)
}

func (s *Server) updateCurrentUser(w http.ResponseWriter, r *http.Request) {
	userID, ok := currentUserID(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	var request UpdateProfileRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeValidationProblem(w, "body", "must be valid JSON")
		return
	}
	if request.DisplayName == nil || utf8.RuneCountInString(*request.DisplayName) < 1 || utf8.RuneCountInString(*request.DisplayName) > 100 {
		writeValidationProblem(w, "displayName", "must contain between 1 and 100 characters")
		return
	}
	if s.currentUsers == nil {
		writeProblem(w, http.StatusServiceUnavailable, "profile-unavailable", "Service Unavailable", "Profile is temporarily unavailable.")
		return
	}
	user, err := s.currentUsers.UpdateDisplayName(r.Context(), userID, *request.DisplayName)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "profile-update-failed", "Internal Server Error", "Profile could not be updated.")
		return
	}
	roles, err := s.currentUsers.ListRolesForUser(r.Context(), userID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "profile-load-failed", "Internal Server Error", "Profile could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, apiUser(user, roles))
}

func (s *Server) loadCurrentUser(ctx context.Context, userID uuid.UUID, w http.ResponseWriter) (store.User, []string, bool) {
	if s.currentUsers == nil {
		writeProblem(w, http.StatusServiceUnavailable, "profile-unavailable", "Service Unavailable", "Profile is temporarily unavailable.")
		return store.User{}, nil, false
	}
	user, err := s.currentUsers.GetUserByID(ctx, userID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "profile-load-failed", "Internal Server Error", "Profile could not be loaded.")
		return store.User{}, nil, false
	}
	roles, err := s.currentUsers.ListRolesForUser(ctx, userID)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "profile-load-failed", "Internal Server Error", "Profile could not be loaded.")
		return store.User{}, nil, false
	}
	return user, roles, true
}

func currentUserID(r *http.Request) (uuid.UUID, bool) {
	claims, ok := ClaimsFrom(r.Context())
	if !ok {
		return uuid.Nil, false
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, false
	}
	return userID, true
}

func apiUser(user store.User, roles []string) User {
	apiRoles := make([]Role, len(roles))
	for index, role := range roles {
		apiRoles[index] = Role(role)
	}
	return User{
		Id:          openapi_types.UUID(user.ID),
		Email:       openapi_types.Email(user.Email),
		DisplayName: user.DisplayName,
		Status:      UserStatus(user.Status),
		Roles:       apiRoles,
		MfaEnabled:  user.MFAEnabled,
		CreatedAt:   user.CreatedAt,
		LastLoginAt: user.LastLoginAt,
	}
}
