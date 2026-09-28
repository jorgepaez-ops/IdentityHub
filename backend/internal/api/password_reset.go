package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jorgepaez/identity-hub/internal/auth/passwordreset"
)

func (s *Server) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	if s.passwordReset == nil {
		writeProblem(w, http.StatusServiceUnavailable, "password-reset-unavailable", "Service Unavailable", "Password reset is temporarily unavailable.")
		return
	}
	var request EmailRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeValidationProblem(w, "body", "must be valid JSON")
		return
	}
	// RF-015 / AM-004: Request never distinguishes a missing account from an
	// existing one. All successful syntactic requests use the same 202+empty body.
	if err := s.passwordReset.Request(r.Context(), string(request.Email)); err != nil && s.logger != nil {
		s.logger.Error("password reset request could not be delivered", "error", err)
	}
	w.WriteHeader(http.StatusAccepted)
}
func (s *Server) confirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	if s.passwordReset == nil {
		writeProblem(w, http.StatusServiceUnavailable, "password-reset-unavailable", "Service Unavailable", "Password reset is temporarily unavailable.")
		return
	}
	var request PasswordResetConfirmRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeValidationProblem(w, "body", "must be valid JSON")
		return
	}
	err := s.passwordReset.Confirm(r.Context(), request.Token, request.Password)
	var invalid *passwordreset.InvalidInputError
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, passwordreset.ErrInvalidInput):
		writeValidationProblem(w, "token", "must not be empty")
	case errors.As(err, &invalid):
		writeValidationProblem(w, invalid.Field, invalid.Detail)
	case errors.Is(err, passwordreset.ErrTokenInvalid):
		writeProblem(w, http.StatusGone, "password-reset-token-unavailable", "Gone", "The password reset token is no longer available.")
	default:
		writeProblem(w, http.StatusInternalServerError, "password-reset-failed", "Internal Server Error", "The password could not be reset.")
	}
}
