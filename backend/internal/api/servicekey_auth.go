package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/jorgepaez/identity-hub/internal/auth/servicekey"
)

const serviceKeyPrincipalKey ctxKey = "serviceKeyPrincipal"

// ServiceKeyPrincipalFrom returns the service key that authenticated the request, if any.
func ServiceKeyPrincipalFrom(ctx context.Context) (servicekey.Principal, bool) {
	principal, ok := ctx.Value(serviceKeyPrincipalKey).(servicekey.Principal)
	return principal, ok
}

// serviceKeyOrAdmin protects an operation that declares a service key scope: a bearer value with the
// ihk_ prefix is authenticated as a service key that must hold scope; anything else follows the
// human path (access token plus admin role) exactly as before. Operations that do not use this
// wrapper never accept a service key: RequireAuth does not recognize the format and answers 401.
func (s *Server) serviceKeyOrAdmin(scope string, next http.HandlerFunc) http.Handler {
	human := s.adminOnly(next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || !strings.HasPrefix(raw, servicekey.KeyPrefix) {
			human.ServeHTTP(w, r)
			return
		}
		if s.serviceKeys == nil {
			writeProblem(w, http.StatusServiceUnavailable, "service-keys-unavailable", titleServiceUnavailable, "Service keys are temporarily unavailable.")
			return
		}
		meta := servicekey.RequestMeta{UserAgent: r.UserAgent()}
		if ip := requestClientIP(r); ip != nil {
			meta.IP = *ip
		}
		principal, err := s.serviceKeys.Authenticate(r.Context(), raw, scope, meta)
		switch {
		case err == nil:
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), serviceKeyPrincipalKey, principal)))
		case errors.Is(err, servicekey.ErrUnauthenticated):
			writeUnauthorized(w)
		case errors.Is(err, servicekey.ErrInsufficientScope):
			writeProblem(w, http.StatusForbidden, "forbidden", "Forbidden", "You do not have permission to access this resource.")
		default:
			s.logServiceKeyError(r, "service key authentication failed", err)
			writeProblem(w, http.StatusInternalServerError, "authorization-load-failed", titleInternalServerError, "Authorization could not be verified.")
		}
	})
}
