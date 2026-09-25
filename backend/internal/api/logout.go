package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jorgepaez/identity-hub/internal/auth/logout"
)

type LogoutService interface {
	Logout(context.Context, logout.Input) error
}

// NewLogoutHandler returns the HTTP boundary for revoking one refresh token.
func NewLogoutHandler(service LogoutService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(refreshCookieName)
		if err != nil || cookie.Value == "" {
			clearRefreshCookie(w)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		err = service.Logout(r.Context(), logout.Input{RefreshToken: cookie.Value, IP: requestClientIP(r), UserAgent: optionalRequestUserAgent(r)})
		if err != nil {
			if errors.Is(err, logout.ErrInvalidRefreshToken) {
				clearRefreshCookie(w)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		clearRefreshCookie(w)
		w.WriteHeader(http.StatusNoContent)
	})
}

func clearRefreshCookie(w http.ResponseWriter) {
	cookie := refreshCookie("")
	cookie.MaxAge = -1
	cookie.Expires = time.Unix(1, 0)
	http.SetCookie(w, cookie)
}
