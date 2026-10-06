package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/mfa"
	"github.com/jorgepaez/identity-hub/internal/observability"
)

// Login verifies the password and starts the mandatory email MFA challenge
// (D11). No session exists until POST /auth/mfa/verify accepts the code.
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	var request LoginRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeValidationProblem(w, "body", "must be valid JSON")
		return
	}
	if request.Email == "" || request.Password == "" {
		writeValidationProblem(w, "body", "email and password are required")
		return
	}
	if s.login == nil {
		writeProblem(w, http.StatusServiceUnavailable, "login-unavailable", "Service Unavailable", "Login is temporarily unavailable.")
		return
	}
	result, err := s.login.Login(r.Context(), login.Input{Email: string(request.Email), Password: request.Password, IP: requestClientIP(r), UserAgent: optionalRequestUserAgent(r)})
	switch {
	case err == nil:
		writeJSON(w, http.StatusAccepted, MfaChallenge{MfaToken: result.MfaToken, ExpiresIn: result.ExpiresIn})
		observability.LoginAttempts.WithLabelValues("mfa_required").Inc()
	case errors.Is(err, login.ErrAccountLocked):
		writeProblem(w, http.StatusLocked, "login-locked", "Locked", "Login is temporarily unavailable. Please try again later.")
		observability.LoginAttempts.WithLabelValues("locked").Inc()
	case errors.Is(err, login.ErrIPRateLimited):
		// Not counted in LoginAttempts: the request is rejected before any credential is
		// checked, so it is not an authentication outcome.
		writeProblem(w, http.StatusLocked, "login-locked", "Locked", "Login is temporarily unavailable. Please try again later.")
	case errors.Is(err, login.ErrInvalidCredentials):
		writeProblem(w, http.StatusUnauthorized, "invalid-credentials", "Unauthorized", "Invalid email or password.")
		// The attempt that triggers a lock also lands here (the domain reports invalid
		// credentials); the following attempts are counted as "locked".
		observability.LoginAttempts.WithLabelValues("failed").Inc()
	case errors.Is(err, mfa.ErrIssuanceLimited):
		writeProblem(w, http.StatusTooManyRequests, "mfa-challenge-rate-limited", "Too Many Requests", "Too many verification codes were requested. Please try again later.")
	case errors.Is(err, mfa.ErrDeliveryUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "mfa-delivery-unavailable", "Service Unavailable", "The verification code could not be sent. Please try again.")
	default:
		if s.logger != nil {
			s.logger.Error("login failed", "error", err, "request_id", TraceIDFrom(r.Context()))
		}
		writeProblem(w, http.StatusInternalServerError, "login-failed", "Internal Server Error", "Login could not be completed.")
	}
}
