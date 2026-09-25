package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jorgepaez/identity-hub/internal/auth/verification"
)

// VerifyEmail consumes a one-time verification token and activates its account.
func (s *Server) VerifyEmail(w http.ResponseWriter, r *http.Request) {
	if s.verification == nil {
		writeProblem(w, http.StatusServiceUnavailable, "verification-unavailable", "Service Unavailable", "Email verification is temporarily unavailable.")
		return
	}
	var request TokenRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeValidationProblem(w, "body", "must be valid JSON")
		return
	}
	err := s.verification.Verify(r.Context(), verification.Input{Token: request.Token, IP: requestClientIP(r), UserAgent: optionalRequestUserAgent(r)})
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, verification.ErrInvalidInput):
		writeValidationProblem(w, "token", "must not be empty")
	case errors.Is(err, verification.ErrTokenInvalid):
		// Unknown, expired, and used tokens deliberately share this response to
		// avoid revealing which token state was observed.
		writeProblem(w, http.StatusGone, "verification-token-unavailable", "Gone", "The verification token is no longer available.")
	case errors.Is(err, verification.ErrPublish):
		writeProblem(w, http.StatusServiceUnavailable, "event-unavailable", "Service Unavailable", "Email verification is temporarily unavailable.")
	default:
		writeProblem(w, http.StatusInternalServerError, "verification-failed", "Internal Server Error", "Email verification could not be completed.")
	}
}
