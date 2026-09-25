package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jorgepaez/identity-hub/internal/auth/login"
)

// Login authenticates an active account and places its opaque refresh token in
// a secure cookie. The token never appears in the JSON response.
func (s *Server) Login(w http.ResponseWriter, r *http.Request) {
	if s.login == nil || s.refreshTTL <= 0 {
		writeProblem(w, http.StatusServiceUnavailable, "login-unavailable", "Service Unavailable", "Login is temporarily unavailable.")
		return
	}
	var request LoginRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeValidationProblem(w, "body", "must be valid JSON")
		return
	}
	result, err := s.login.Login(r.Context(), login.Input{Email: string(request.Email), Password: request.Password, IP: requestClientIP(r), UserAgent: optionalRequestUserAgent(r)})
	switch {
	case err == nil:
		http.SetCookie(w, &http.Cookie{Name: "refresh_token", Value: result.RefreshToken, Path: "/api/v1/auth", MaxAge: int(s.refreshTTL.Seconds()), HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode})
		writeJSON(w, http.StatusOK, TokenPair{AccessToken: result.AccessToken, TokenType: TokenPairTokenType(result.TokenType), ExpiresIn: result.ExpiresIn})
	case errors.Is(err, login.ErrMFAUnavailable):
		writeProblem(w, http.StatusNotImplemented, "mfa-not-supported", "Not Implemented", "A second authentication factor is required, but MFA is not supported yet.")
	case errors.Is(err, login.ErrInvalidCredentials):
		writeProblem(w, http.StatusUnauthorized, "invalid-credentials", "Unauthorized", "Invalid email or password.")
	default:
		writeProblem(w, http.StatusInternalServerError, "login-failed", "Internal Server Error", "Login could not be completed.")
	}
}
