package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/netip"

	"github.com/jorgepaez/identity-hub/internal/auth/registration"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// Register creates a pending account and publishes its verification event.
func (s *Server) Register(w http.ResponseWriter, r *http.Request) {
	if s.registration == nil {
		writeProblem(w, http.StatusServiceUnavailable, "registration-unavailable", "Service Unavailable", "Registration is temporarily unavailable.")
		return
	}
	var request RegisterRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeValidationProblem(w, "body", "must be valid JSON")
		return
	}
	result, err := s.registration.Register(r.Context(), registration.Input{Email: string(request.Email), Password: request.Password, DisplayName: request.DisplayName, IP: requestClientIP(r), UserAgent: optionalRequestUserAgent(r)})
	if err != nil {
		var invalid *registration.InvalidInputError
		switch {
		case errors.As(err, &invalid):
			writeValidationProblem(w, invalid.Field, invalid.Detail)
		case errors.Is(err, registration.ErrEmailExists):
			writeProblem(w, http.StatusConflict, "registration-conflict", "Conflict", "A registration request cannot be completed.")
		case errors.Is(err, registration.ErrPublish):
			writeProblem(w, http.StatusServiceUnavailable, "event-unavailable", "Service Unavailable", "Registration is temporarily unavailable.")
		default:
			writeProblem(w, http.StatusInternalServerError, "registration-failed", "Internal Server Error", "Registration could not be completed.")
		}
		return
	}
	writeJSON(w, http.StatusCreated, RegisterResponse{Id: openapi_types.UUID(result.ID), Email: openapi_types.Email(result.Email), Status: UserStatus(result.Status)})
}

func optionalRequestUserAgent(r *http.Request) *string {
	if value := r.UserAgent(); value != "" {
		return &value
	}
	return nil
}

func writeValidationProblem(w http.ResponseWriter, field, detail string) {
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]any{"type": "https://identity.local/problems/validation", "title": "Bad Request", "status": http.StatusBadRequest, "detail": "Request validation failed.", "errors": []map[string]string{{"field": field, "detail": detail}}})
}

func requestClientIP(r *http.Request) *netip.Addr {
	address, ok := ClientIPFrom(r.Context())
	if !ok {
		return nil
	}
	return &address
}
