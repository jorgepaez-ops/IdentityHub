package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jorgepaez/identity-hub/internal/auth/employee"
	"github.com/jorgepaez/identity-hub/internal/store"
)

// createEmployee implements POST /api/v1/admin/users (RF-001, D6/D9). The
// caller's admin role was already verified by CreateEmployee's RequireRole
// wrapper; the admin never sets or learns the new account's password.
func (s *Server) createEmployee(w http.ResponseWriter, r *http.Request) {
	if s.employeeCreation == nil {
		writeProblem(w, http.StatusServiceUnavailable, "employee-creation-unavailable", titleServiceUnavailable, "Employee creation is temporarily unavailable.")
		return
	}
	actorID, ok := currentUserID(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	var request AdminCreateUserRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeValidationProblem(w, "body", "must be valid JSON")
		return
	}
	roleNames := make([]string, len(request.Roles))
	for i, role := range request.Roles {
		roleNames[i] = string(role)
	}
	result, err := s.employeeCreation.CreateEmployee(r.Context(), employee.Input{
		Email:       string(request.Email),
		DisplayName: request.DisplayName,
		Roles:       roleNames,
		ActorUserID: actorID,
		IP:          requestClientIP(r),
		UserAgent:   optionalRequestUserAgent(r),
	})
	if err != nil {
		var invalid *employee.InvalidInputError
		switch {
		case errors.As(err, &invalid):
			writeValidationProblem(w, invalid.Field, invalid.Detail)
		case errors.Is(err, employee.ErrEmailExists):
			// Deliberately as uninformative as the registration conflict
			// response used to be: it must not reveal that the account exists.
			writeProblem(w, http.StatusConflict, "employee-conflict", "Conflict", "An employee account cannot be created.")
		case errors.Is(err, employee.ErrPublish):
			writeProblem(w, http.StatusServiceUnavailable, "event-unavailable", titleServiceUnavailable, "Employee creation is temporarily unavailable.")
		default:
			writeProblem(w, http.StatusInternalServerError, "employee-creation-failed", titleInternalServerError, "The employee account could not be created.")
		}
		return
	}
	writeJSON(w, http.StatusCreated, apiEmployeeResult(result))
}

// apiEmployeeResult reuses apiUser (me.go), the same store.User-to-API
// conversion admin_users.go's apiAdminUser uses, so the created-employee
// response shape matches every other User body in the contract.
func apiEmployeeResult(result employee.Result) User {
	return apiUser(store.User{
		ID:          result.ID,
		Email:       result.Email,
		DisplayName: result.DisplayName,
		Status:      result.Status,
		CreatedAt:   result.CreatedAt,
	}, result.Roles)
}
