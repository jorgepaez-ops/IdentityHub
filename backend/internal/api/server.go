// Package api expone el servidor HTTP.
//
// A partir de la semana 2 los handlers implementan la interfaz generada por
// oapi-codegen desde specs/03-api/openapi.yaml, de modo que divergir del
// contrato pasa a ser un error de compilación y no un fallo en producción.
package api

import (
	"log/slog"
	"net/http"
	"net/netip"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/jorgepaez/identity-hub/internal/auth/registration"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

type Server struct {
	logger         *slog.Logger
	version        string
	deps           map[string]Checker
	tokens         *token.Service
	registration   registration.Registrar
	trustedProxies []netip.Prefix
}

func NewServer(logger *slog.Logger, version string, deps map[string]Checker) *Server {
	return &Server{logger: logger, version: version, deps: deps}
}

func (s *Server) SetTokenService(tokens *token.Service) { s.tokens = tokens }

// SetRegistrationService is used by composition and focused handler tests.
func (s *Server) SetRegistrationService(service registration.Registrar) { s.registration = service }

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

	return HandlerFromMux(s, r)
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
func (s *Server) Login(w http.ResponseWriter, r *http.Request) { s.notImplemented(w) }
func (s *Server) Logout(w http.ResponseWriter, r *http.Request, params LogoutParams) {
	s.notImplemented(w)
}
func (s *Server) VerifyMfa(w http.ResponseWriter, r *http.Request)            { s.notImplemented(w) }
func (s *Server) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) { s.notImplemented(w) }
func (s *Server) RequestPasswordReset(w http.ResponseWriter, r *http.Request) { s.notImplemented(w) }
func (s *Server) RefreshSession(w http.ResponseWriter, r *http.Request, params RefreshSessionParams) {
	s.notImplemented(w)
}

func (s *Server) VerifyEmail(w http.ResponseWriter, r *http.Request)       { s.notImplemented(w) }
func (s *Server) GetCurrentUser(w http.ResponseWriter, r *http.Request)    { s.notImplemented(w) }
func (s *Server) UpdateCurrentUser(w http.ResponseWriter, r *http.Request) { s.notImplemented(w) }
func (s *Server) DisableMfa(w http.ResponseWriter, r *http.Request)        { s.notImplemented(w) }
func (s *Server) ActivateMfa(w http.ResponseWriter, r *http.Request)       { s.notImplemented(w) }
func (s *Server) EnrollMfa(w http.ResponseWriter, r *http.Request)         { s.notImplemented(w) }
func (s *Server) ListSessions(w http.ResponseWriter, r *http.Request)      { s.notImplemented(w) }
func (s *Server) RevokeSession(w http.ResponseWriter, r *http.Request, sessionID SessionId) {
	s.notImplemented(w)
}
func (s *Server) GetHealth(w http.ResponseWriter, r *http.Request)    { s.Health(w, r) }
func (s *Server) GetReadiness(w http.ResponseWriter, r *http.Request) { s.Readiness(w, r) }

func (s *Server) notImplemented(w http.ResponseWriter) {
	writeProblem(w, http.StatusNotImplemented, "not-implemented", "Not Implemented", "This operation is not implemented yet.")
}
