package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/rolegrid"
)

// Handlers of the configurable role grid (RF-021, ADR 0013). They are reached
// only through the Server methods wrapped with RequireRole("admin").

func (s *Server) listApplications(w http.ResponseWriter, r *http.Request) {
	if !s.roleGridReady(w) {
		return
	}
	applications, err := s.roleGrid.ListApplications(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "applications-load-failed", "Internal Server Error", "Applications could not be loaded.")
		return
	}
	result := make([]Application, 0, len(applications))
	for _, application := range applications {
		result = append(result, apiApplication(application))
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) listApplicationRoles(w http.ResponseWriter, r *http.Request, applicationID ApplicationId) {
	if !s.roleGridReady(w) {
		return
	}
	roles, err := s.roleGrid.ListRoles(r.Context(), uuid.UUID(applicationID))
	if err != nil {
		s.writeRoleGridError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiApplicationRoles(roles))
}

func (s *Server) createApplicationRole(w http.ResponseWriter, r *http.Request, applicationID ApplicationId) {
	if !s.roleGridReady(w) {
		return
	}
	actor, ok := currentUserID(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	var request CreateApplicationRoleRequest
	if !decodeRoleGridRequest(w, r, &request) {
		return
	}
	input := rolegrid.CreateInput{ActorUserID: actor, ApplicationID: uuid.UUID(applicationID), Name: request.Name, PermissionKeys: request.PermissionKeys, UserAgent: r.UserAgent()}
	if request.Description != nil {
		input.Description = *request.Description
	}
	if ip := requestClientIP(r); ip != nil {
		input.IP = *ip
	}
	role, err := s.roleGrid.Create(r.Context(), input)
	if err != nil {
		s.writeRoleGridError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, apiApplicationRole(role))
}

func (s *Server) updateApplicationRole(w http.ResponseWriter, r *http.Request, applicationID ApplicationId, roleID RoleId) {
	if !s.roleGridReady(w) {
		return
	}
	actor, ok := currentUserID(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	var request UpdateApplicationRoleRequest
	if !decodeRoleGridRequest(w, r, &request) {
		return
	}
	if request.Description == nil && request.PermissionKeys == nil {
		writeValidationProblem(w, "body", "must include description or permissionKeys")
		return
	}
	input := rolegrid.UpdateInput{ActorUserID: actor, ApplicationID: uuid.UUID(applicationID), RoleID: uuid.UUID(roleID), Description: request.Description, PermissionKeys: request.PermissionKeys, UserAgent: r.UserAgent()}
	if ip := requestClientIP(r); ip != nil {
		input.IP = *ip
	}
	role, err := s.roleGrid.Update(r.Context(), input)
	if err != nil {
		s.writeRoleGridError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, apiApplicationRole(role))
}

func (s *Server) deleteApplicationRole(w http.ResponseWriter, r *http.Request, applicationID ApplicationId, roleID RoleId) {
	if !s.roleGridReady(w) {
		return
	}
	actor, ok := currentUserID(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	input := rolegrid.DeleteInput{ActorUserID: actor, ApplicationID: uuid.UUID(applicationID), RoleID: uuid.UUID(roleID), UserAgent: r.UserAgent()}
	if ip := requestClientIP(r); ip != nil {
		input.IP = *ip
	}
	if err := s.roleGrid.Delete(r.Context(), input); err != nil {
		s.writeRoleGridError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) roleGridReady(w http.ResponseWriter) bool {
	if s.roleGrid == nil {
		writeProblem(w, http.StatusServiceUnavailable, "role-grid-unavailable", "Service Unavailable", "Role management is temporarily unavailable.")
		return false
	}
	return true
}

// decodeRoleGridRequest rejects unknown fields, so a rename ("name" on PATCH) is a 400.
func decodeRoleGridRequest(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeValidationProblem(w, "body", "must be valid JSON without unknown fields")
		return false
	}
	return true
}

func (s *Server) writeRoleGridError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, rolegrid.ErrInvalidRoleName):
		writeProblem(w, http.StatusBadRequest, "invalid-application-role", "Bad Request", "The role name must be <application>.<name> with lowercase letters, digits, '_' or '-'.")
	case errors.Is(err, rolegrid.ErrInvalidPermission):
		writeProblem(w, http.StatusBadRequest, "invalid-application-role", "Bad Request", "A permission is unknown for this application.")
	case errors.Is(err, rolegrid.ErrInvalidDescription), errors.Is(err, rolegrid.ErrEmptyUpdate):
		writeProblem(w, http.StatusBadRequest, "invalid-application-role", "Bad Request", "The role description or update is invalid.")
	case errors.Is(err, rolegrid.ErrSelfPermissionChange), errors.Is(err, rolegrid.ErrSelfRoleDelete):
		writeProblem(w, http.StatusForbidden, "role-change-forbidden", "Forbidden", "An administrator cannot change or delete a role they hold.")
	case errors.Is(err, rolegrid.ErrSystemRole):
		writeProblem(w, http.StatusForbidden, "role-change-forbidden", "Forbidden", "System roles cannot be changed.")
	case errors.Is(err, rolegrid.ErrApplicationNotFound):
		writeProblem(w, http.StatusNotFound, "application-not-found", "Not Found", "The application was not found.")
	case errors.Is(err, rolegrid.ErrRoleNotFound):
		writeProblem(w, http.StatusNotFound, "application-role-not-found", "Not Found", "The application role was not found.")
	case errors.Is(err, rolegrid.ErrDuplicateRole):
		writeProblem(w, http.StatusConflict, "application-role-conflict", "Conflict", "A role with that name already exists.")
	case errors.Is(err, rolegrid.ErrRoleAssigned):
		writeProblem(w, http.StatusConflict, "application-role-conflict", "Conflict", "The role is assigned to users and cannot be deleted.")
	default:
		writeProblem(w, http.StatusInternalServerError, "application-role-change-failed", "Internal Server Error", "The role operation could not be completed.")
	}
}

func apiApplication(application rolegrid.Application) Application {
	result := Application{Id: application.ID, ClientId: application.ClientID, Name: application.Name, Permissions: make([]Permission, 0, len(application.Permissions)), Roles: apiApplicationRoles(application.Roles)}
	for _, permission := range application.Permissions {
		result.Permissions = append(result.Permissions, Permission{Key: permission.Key, Description: permission.Description})
	}
	return result
}

func apiApplicationRoles(roles []rolegrid.Role) []ApplicationRole {
	result := make([]ApplicationRole, 0, len(roles))
	for _, role := range roles {
		result = append(result, apiApplicationRole(role))
	}
	return result
}

func apiApplicationRole(role rolegrid.Role) ApplicationRole {
	return ApplicationRole{Id: role.ID, ApplicationId: role.ApplicationID, Name: role.Name, Description: role.Description, PermissionKeys: append([]string{}, role.PermissionKeys...), System: role.System, AssignedCount: int(role.AssignedCount)}
}
