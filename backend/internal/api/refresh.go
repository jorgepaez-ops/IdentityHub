package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jorgepaez/identity-hub/internal/auth/refresh"
)

const refreshCookieName = "refresh_token"

type RefreshService interface {
	Refresh(context.Context, refresh.Input) (refresh.Result, error)
}

// NewRefreshHandler returns the HTTP boundary for refresh rotation.
func NewRefreshHandler(service RefreshService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(refreshCookieName)
		if err != nil || cookie.Value == "" {
			unauthorized(w, false)
			return
		}
		result, err := service.Refresh(r.Context(), refresh.Input{RefreshToken: cookie.Value, IP: requestClientIP(r), UserAgent: optionalRequestUserAgent(r)})
		if err != nil {
			if errors.Is(err, refresh.ErrInvalidRefreshToken) || errors.Is(err, refresh.ErrRefreshReuse) {
				unauthorized(w, true)
				return
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, refreshCookie(result.RefreshToken))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(struct {
			AccessToken string `json:"accessToken"`
			TokenType   string `json:"tokenType"`
			ExpiresIn   int    `json:"expiresIn"`
		}{result.AccessToken, result.TokenType, result.ExpiresIn})
	})
}

func refreshCookie(value string) *http.Cookie {
	return &http.Cookie{Name: refreshCookieName, Value: value, Path: "/api/v1/auth", HttpOnly: true, Secure: true, SameSite: http.SameSiteStrictMode}
}

func unauthorized(w http.ResponseWriter, clear bool) {
	if clear {
		cookie := refreshCookie("")
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0)
		http.SetCookie(w, cookie)
	}
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}
