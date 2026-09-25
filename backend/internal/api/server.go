// Package api expone el servidor HTTP.
//
// A partir de la semana 2 los handlers implementan la interfaz generada por
// oapi-codegen desde specs/03-api/openapi.yaml, de modo que divergir del
// contrato pasa a ser un error de compilación y no un fallo en producción.
package api

import (
	"errors"
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/logout"
	"github.com/jorgepaez/identity-hub/internal/auth/refresh"
	"github.com/jorgepaez/identity-hub/internal/auth/registration"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/auth/verification"
)

type Server struct {
	logger         *slog.Logger
	version        string
	deps           map[string]Checker
	tokens         *token.Service
	registration   registration.Registrar
	login          login.Authenticator
	refreshTTL     time.Duration
	verification   verification.Verifier
	currentUsers   currentUserRepository
	refresh        refresh.Refresher
	logout         logout.Revoker
	trustedProxies []netip.Prefix
}

func NewServer(logger *slog.Logger, version string, deps map[string]Checker) *Server {
	return &Server{logger: logger, version: version, deps: deps}
}

func (s *Server) SetTokenService(tokens *token.Service) { s.tokens = tokens }

// SetRegistrationService is used by composition and focused handler tests.
func (s *Server) SetRegistrationService(service registration.Registrar) { s.registration = service }

// SetLoginService is used by composition and focused handler tests.
func (s *Server) SetLoginService(service login.Authenticator, refreshTTL time.Duration) {
	s.login = service
	s.refreshTTL = refreshTTL
}

// SetEmailVerificationService is used by composition and focused handler tests.
func (s *Server) SetEmailVerificationService(service verification.Verifier) { s.verification = service }

// SetCurrentUserRepository is used by composition and focused profile tests.
func (s *Server) SetCurrentUserRepository(repository currentUserRepository) {
	s.currentUsers = repository
}

// SetRefreshService is used by composition and focused handler tests.
func (s *Server) SetRefreshService(service refresh.Refresher) { s.refresh = service }

// SetLogoutService is used by composition and focused handler tests.
func (s *Server) SetLogoutService(service logout.Revoker) { s.logout = service }

func (s *Server) SetTrustedProxies(prefixes []netip.Prefix) {
	s.trustedProxies = append([]netip.Prefix(nil), prefixes...)
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Use(ClientIP(s.trustedProxies))
	r.Use(TraceID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	// Un cuerpo de petición acotado (AM-018). Ninguna operación del contrato
	// necesita más de 1 MiB.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
			next.ServeHTTP(w, req)
		})
	})
	r.Use(Metrics)

	// /metrics no se expone al exterior: Nginx no lo proxea y en producción el
	// puerto de la API no se publica al host. Solo Prometheus, dentro de la red
	// de Docker, puede alcanzarlo.
	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		// ⚠️ Endpoint de la línea base vulnerable. No está en el contrato
		// OpenAPI y se elimina en la fase de remediación (ver adr/0007).
		r.Get("/auth/legacy-login", s.LegacyLogin)
	})

	return HandlerWithOptions(s, ChiServerOptions{BaseRouter: r, ErrorHandlerFunc: s.handleBindingError})
}

// handleBindingError overrides oapi-codegen's default binding error response
// (a plain 400) for the refresh_token cookie parameter: a missing or
// malformed cookie on /auth/refresh and /auth/logout must behave exactly
// like an invalid token (401, with the compromised cookie cleared), per
// RF-005/RF-007, not surface as a generic bad request.
func (s *Server) handleBindingError(w http.ResponseWriter, r *http.Request, err error) {
	var paramName string
	var required *RequiredParamError
	var invalidFormat *InvalidParamFormatError
	switch {
	case errors.As(err, &required):
		paramName = required.ParamName
	case errors.As(err, &invalidFormat):
		paramName = invalidFormat.ParamName
	}
	if paramName == refreshCookieName {
		clearRefreshCookie(w)
		writeUnauthorized(w)
		return
	}
	http.Error(w, err.Error(), http.StatusBadRequest)
}

func (s *Server) ListAuditLog(w http.ResponseWriter, r *http.Request, params ListAuditLogParams) {
	s.notImplemented(w)
}
func (s *Server) ListUsers(w http.ResponseWriter, r *http.Request, params ListUsersParams) {
	s.notImplemented(w)
}
func (s *Server) GetUser(w http.ResponseWriter, r *http.Request, userID UserId) { s.notImplemented(w) }
func (s *Server) UpdateUser(w http.ResponseWriter, r *http.Request, userID UserId) {
	s.notImplemented(w)
}
func (s *Server) VerifyMfa(w http.ResponseWriter, r *http.Request)            { s.notImplemented(w) }
func (s *Server) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) { s.notImplemented(w) }
func (s *Server) RequestPasswordReset(w http.ResponseWriter, r *http.Request) { s.notImplemented(w) }

func (s *Server) DisableMfa(w http.ResponseWriter, r *http.Request)   { s.notImplemented(w) }
func (s *Server) ActivateMfa(w http.ResponseWriter, r *http.Request)  { s.notImplemented(w) }
func (s *Server) EnrollMfa(w http.ResponseWriter, r *http.Request)    { s.notImplemented(w) }
func (s *Server) ListSessions(w http.ResponseWriter, r *http.Request) { s.notImplemented(w) }
func (s *Server) RevokeSession(w http.ResponseWriter, r *http.Request, sessionID SessionId) {
	s.notImplemented(w)
}
func (s *Server) GetHealth(w http.ResponseWriter, r *http.Request)    { s.Health(w, r) }
func (s *Server) GetReadiness(w http.ResponseWriter, r *http.Request) { s.Readiness(w, r) }

func (s *Server) notImplemented(w http.ResponseWriter) {
	writeProblem(w, http.StatusNotImplemented, "not-implemented", "Not Implemented", "This operation is not implemented yet.")
}
