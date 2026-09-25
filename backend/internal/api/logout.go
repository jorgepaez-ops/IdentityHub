package api

import (
	"errors"
	"net/http"

	"github.com/jorgepaez/identity-hub/internal/auth/logout"
)

// Logout revokes the refresh token presented in the refresh_token cookie. A
// missing or malformed cookie never reaches this method: the generated
// router rejects it earlier through handleBindingError, which maps it to the
// same 401 response as an unknown or already revoked token below.
func (s *Server) Logout(w http.ResponseWriter, r *http.Request, params LogoutParams) {
	if s.logout == nil {
		writeProblem(w, http.StatusServiceUnavailable, "logout-unavailable", "Service Unavailable", "Logout is temporarily unavailable.")
		return
	}
	err := s.logout.Logout(r.Context(), logout.Input{RefreshToken: params.RefreshToken, IP: requestClientIP(r), UserAgent: optionalRequestUserAgent(r)})
	if err != nil {
		if errors.Is(err, logout.ErrInvalidRefreshToken) {
			clearRefreshCookie(w)
			writeUnauthorized(w)
			return
		}
		writeProblem(w, http.StatusInternalServerError, "logout-failed", "Internal Server Error", "Logout could not be completed.")
		return
	}
	clearRefreshCookie(w)
	w.WriteHeader(http.StatusNoContent)
}
