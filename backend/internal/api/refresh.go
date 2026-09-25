package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/jorgepaez/identity-hub/internal/auth/refresh"
)

const refreshCookieName = "refresh_token"

// RefreshSession rotates the refresh token presented in the refresh_token
// cookie. A missing or malformed cookie never reaches this method: the
// generated router rejects it earlier through handleBindingError, which
// maps it to the same 401 response as an invalid or reused token below.
func (s *Server) RefreshSession(w http.ResponseWriter, r *http.Request, params RefreshSessionParams) {
	if s.refresh == nil {
		writeProblem(w, http.StatusServiceUnavailable, "refresh-unavailable", "Service Unavailable", "Refresh is temporarily unavailable.")
		return
	}
	result, err := s.refresh.Refresh(r.Context(), refresh.Input{RefreshToken: params.RefreshToken, IP: requestClientIP(r), UserAgent: optionalRequestUserAgent(r)})
	if err != nil {
		if errors.Is(err, refresh.ErrInvalidRefreshToken) || errors.Is(err, refresh.ErrRefreshReuse) {
			clearRefreshCookie(w)
			writeUnauthorized(w)
			return
		}
		writeProblem(w, http.StatusInternalServerError, "refresh-failed", "Internal Server Error", "The session could not be refreshed.")
		return
	}
	http.SetCookie(w, refreshCookie(result.RefreshToken))
	writeJSON(w, http.StatusOK, TokenPair{AccessToken: result.AccessToken, TokenType: TokenPairTokenType(result.TokenType), ExpiresIn: result.ExpiresIn})
}

func refreshCookie(value string) *http.Cookie {
	return &http.Cookie{Name: refreshCookieName, Value: value, Path: "/api/v1/auth", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}
}

func clearRefreshCookie(w http.ResponseWriter) {
	cookie := refreshCookie("")
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(1, 0)
	http.SetCookie(w, cookie)
}
