package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jorgepaez/identity-hub/internal/auth/mfa"
)

func (s *Server) verifyMfa(w http.ResponseWriter, r *http.Request) {
	if s.mfa == nil || s.mfaRefreshTTL <= 0 {
		writeProblem(w, http.StatusServiceUnavailable, "mfa-unavailable", "Service Unavailable", "MFA is temporarily unavailable.")
		return
	}
	var request MfaVerifyRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeValidationProblem(w, "body", "must be valid JSON")
		return
	}
	if request.MfaToken == "" {
		writeValidationProblem(w, "mfaToken", "must not be empty")
		return
	}
	if request.Code == "" {
		writeValidationProblem(w, "code", "must not be empty")
		return
	}
	result, err := s.mfa.Verify(r.Context(), mfa.VerifyInput{
		Token:     request.MfaToken,
		Code:      request.Code,
		IP:        requestClientIP(r),
		UserAgent: optionalRequestUserAgent(r),
	})
	if err == nil {
		// Persist the session across browser restarts, as the pre-MFA login did.
		http.SetCookie(w, refreshCookie(result.RefreshToken, int(s.mfaRefreshTTL.Seconds())))
		writeJSON(w, http.StatusOK, TokenPair{AccessToken: result.AccessToken, TokenType: TokenPairTokenType(result.TokenType), ExpiresIn: result.ExpiresIn})
		return
	}
	if errors.Is(err, mfa.ErrChallengeInvalid) || errors.Is(err, mfa.ErrCodeInvalid) {
		writeUnauthorized(w)
		return
	}
	writeProblem(w, http.StatusInternalServerError, "mfa-verify-failed", "Internal Server Error", "The MFA challenge could not be verified.")
}

func (s *Server) resendMfaCode(w http.ResponseWriter, r *http.Request) {
	if s.mfa == nil {
		writeProblem(w, http.StatusServiceUnavailable, "mfa-unavailable", "Service Unavailable", "MFA is temporarily unavailable.")
		return
	}
	var request MfaResendRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeValidationProblem(w, "body", "must be valid JSON")
		return
	}
	if request.MfaToken == "" {
		writeValidationProblem(w, "mfaToken", "must not be empty")
		return
	}
	err := s.mfa.Resend(r.Context(), request.MfaToken)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusAccepted)
	case errors.Is(err, mfa.ErrResendTooSoon):
		writeProblem(w, http.StatusTooManyRequests, "mfa-resend-rate-limited", "Too Many Requests", "Please wait before requesting another code.")
	case errors.Is(err, mfa.ErrChallengeInvalid):
		writeUnauthorized(w)
	case errors.Is(err, mfa.ErrDeliveryUnavailable):
		writeProblem(w, http.StatusServiceUnavailable, "mfa-delivery-unavailable", "Service Unavailable", "The verification code could not be sent. Please try again.")
	default:
		writeProblem(w, http.StatusInternalServerError, "mfa-resend-failed", "Internal Server Error", "The MFA code could not be resent.")
	}
}
