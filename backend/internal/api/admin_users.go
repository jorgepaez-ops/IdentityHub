package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/admin"
	"github.com/jorgepaez/identity-hub/internal/store"
)

const defaultAdminUsersLimit = 25

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request, params ListUsersParams) {
	if s.adminUsers == nil {
		writeProblem(w, http.StatusServiceUnavailable, "admin-users-unavailable", "Service Unavailable", "User management is temporarily unavailable.")
		return
	}
	limit := defaultAdminUsersLimit
	if params.Limit != nil {
		limit = int(*params.Limit)
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	var status *admin.Status
	if params.Status != nil {
		value := admin.Status(*params.Status)
		status = &value
	}
	var cursor *uuid.UUID
	if params.Cursor != nil {
		value, err := uuid.Parse(string(*params.Cursor))
		if err != nil {
			writeValidationProblem(w, "cursor", "must be a UUID")
			return
		}
		cursor = &value
	}
	users, err := s.adminUsers.ListUsers(r.Context(), admin.ListInput{Query: stringValue(params.Q), Status: status, Cursor: cursor, Limit: limit + 1})
	if errors.Is(err, admin.ErrInvalidStatus) {
		writeValidationProblem(w, "status", "must be a valid user status")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "admin-users-load-failed", "Internal Server Error", "Users could not be loaded.")
		return
	}
	page := UserPage{Items: make([]User, 0, len(users))}
	if len(users) > limit && limit > 0 {
		users = users[:limit]
		cursorValue := users[len(users)-1].ID.String()
		page.NextCursor = &cursorValue
	}
	for _, user := range users {
		page.Items = append(page.Items, apiAdminUser(user))
	}
	writeJSON(w, http.StatusOK, page)
}

func (s *Server) getUser(w http.ResponseWriter, r *http.Request, userID UserId) {
	if s.adminUsers == nil {
		writeProblem(w, http.StatusServiceUnavailable, "admin-users-unavailable", "Service Unavailable", "User management is temporarily unavailable.")
		return
	}
	user, err := s.adminUsers.GetUser(r.Context(), uuid.UUID(userID))
	if errors.Is(err, admin.ErrUserNotFound) || errors.Is(err, pgx.ErrNoRows) {
		writeProblem(w, http.StatusNotFound, "user-not-found", "Not Found", "The user was not found.")
		return
	}
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "admin-user-load-failed", "Internal Server Error", "User could not be loaded.")
		return
	}
	writeJSON(w, http.StatusOK, apiAdminUser(user))
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request, userID UserId) {
	if s.adminUsers == nil {
		writeProblem(w, http.StatusServiceUnavailable, "admin-users-unavailable", "Service Unavailable", "User management is temporarily unavailable.")
		return
	}
	actorID, ok := currentUserID(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	var request AdminUpdateUserRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeValidationProblem(w, "body", "must be valid JSON")
		return
	}
	if request.Status == nil && request.Roles == nil {
		writeValidationProblem(w, "body", "must include status or roles")
		return
	}
	var status *admin.Status
	if request.Status != nil {
		value := admin.Status(*request.Status)
		status = &value
	}
	var roles *[]string
	if request.Roles != nil {
		values := make([]string, len(*request.Roles))
		for i, role := range *request.Roles {
			values[i] = string(role)
		}
		roles = &values
	}
	user, err := s.adminUsers.UpdateUser(r.Context(), admin.UpdateInput{ActorUserID: actorID, UserID: uuid.UUID(userID), Status: status, Roles: roles})
	switch {
	case errors.Is(err, admin.ErrSelfDisable), errors.Is(err, admin.ErrLastActiveAdmin), errors.Is(err, admin.ErrInvalidStatus), errors.Is(err, admin.ErrInvalidRole):
		writeProblem(w, http.StatusBadRequest, "invalid-user-update", "Bad Request", "The requested user update is not allowed.")
	case errors.Is(err, admin.ErrUserNotFound), errors.Is(err, pgx.ErrNoRows):
		writeProblem(w, http.StatusNotFound, "user-not-found", "Not Found", "The user was not found.")
	case err != nil:
		writeProblem(w, http.StatusInternalServerError, "admin-user-update-failed", "Internal Server Error", "User could not be updated.")
	default:
		writeJSON(w, http.StatusOK, apiAdminUser(user))
	}
}

func stringValue(value *Query) string {
	if value == nil {
		return ""
	}
	return string(*value)
}
func apiAdminUser(user admin.User) User {
	return apiUser(store.User{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, Status: string(user.Status), MFAEnabled: user.MFAEnabled, LastLoginAt: user.LastLoginAt, CreatedAt: user.CreatedAt}, user.Roles)
}
