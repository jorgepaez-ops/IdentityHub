package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

const authClaimsKey ctxKey = "authClaims"

func RequireAuth(tokens *token.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tokens == nil {
				writeUnauthorized(w)
				return
			}
			raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			if !ok || raw == "" {
				writeUnauthorized(w)
				return
			}
			claims, err := tokens.Validate(raw)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authClaimsKey, claims)))
		})
	}
}

func ClaimsFrom(ctx context.Context) (token.Claims, bool) {
	claims, ok := ctx.Value(authClaimsKey).(token.Claims)
	return claims, ok
}

func writeUnauthorized(w http.ResponseWriter) {
	writeProblem(w, http.StatusUnauthorized, "unauthorized", "Unauthorized", "Authentication is required.")
}
