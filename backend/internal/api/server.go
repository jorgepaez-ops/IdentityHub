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
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/jorgepaez/identity-hub/internal/auth/admin"
	"github.com/jorgepaez/identity-hub/internal/auth/auditlog"
	"github.com/jorgepaez/identity-hub/internal/auth/employee"
	"github.com/jorgepaez/identity-hub/internal/auth/invitation"
	"github.com/jorgepaez/identity-hub/internal/auth/invitationresend"
	"github.com/jorgepaez/identity-hub/internal/auth/login"
	"github.com/jorgepaez/identity-hub/internal/auth/logout"
	"github.com/jorgepaez/identity-hub/internal/auth/mfa"
	"github.com/jorgepaez/identity-hub/internal/auth/oauth"
	"github.com/jorgepaez/identity-hub/internal/auth/passwordreset"
	"github.com/jorgepaez/identity-hub/internal/auth/refresh"
	"github.com/jorgepaez/identity-hub/internal/auth/rolegrid"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

type Server struct {
	logger           *slog.Logger
	version          string
	deps             map[string]Checker
	tokens           *token.Service
	employeeCreation employee.Creator
	invitationAccept invitation.Acceptor
	invitationResend invitationresend.Resender
	passwordReset    passwordreset.HandlerService
	login            login.Authenticator
	mfa              mfa.Authenticator
	mfaRefreshTTL    time.Duration
	hubSessionTTL    time.Duration
	oauth            *oauth.Service
	hubSessions      oauth.HubSessionReader
	oauthClient      oauth.Client
	oauthRedirect    *url.URL
	oauthLoginURL    string
	currentUsers     currentUserRepository
	refresh          refresh.Refresher
	sessions         sessionManager
	logout           logout.Revoker
	adminUsers       admin.Manager
	roleGrid         rolegrid.Manager
	auditLog         auditlog.Reader
	trustedProxies   []netip.Prefix
}

func NewServer(logger *slog.Logger, version string, deps map[string]Checker) *Server {
	return &Server{logger: logger, version: version, deps: deps}
}

func (s *Server) SetTokenService(tokens *token.Service) { s.tokens = tokens }

// SetEmployeeCreationService is used by composition and focused handler tests
// (T5, RF-001). It replaces the week-2 SetRegistrationService: D9 retired
// public self-registration, so only an admin-driven employee creation
// service is wired into the Server now.
func (s *Server) SetEmployeeCreationService(service employee.Creator) { s.employeeCreation = service }

// SetInvitationAcceptanceService is used by composition and focused handler
// tests (T5, RF-002). It replaces the week-2 SetEmailVerificationService:
// accepting the invitation now does what email verification used to.
func (s *Server) SetInvitationAcceptanceService(service invitation.Acceptor) {
	s.invitationAccept = service
}

func (s *Server) SetInvitationResendService(service invitationresend.Resender) {
	s.invitationResend = service
}
func (s *Server) SetPasswordResetService(service passwordreset.HandlerService) {
	s.passwordReset = service
}

// SetLoginService is used by composition and focused handler tests. The
// password step issues no session, so it needs no refresh lifetime.
func (s *Server) SetLoginService(service login.Authenticator) { s.login = service }

// SetMFAService wires the second step, which issues the session; refreshTTL is
// the lifetime of the refresh cookie it sets.
func (s *Server) SetMFAService(service mfa.Authenticator, refreshTTL time.Duration) {
	s.mfa, s.mfaRefreshTTL = service, refreshTTL
}

// SetOAuthService wires the OAuth endpoints. An unparsable or relative
// redirect URI leaves OAuth unavailable (503) instead of failing per request.
func (s *Server) SetOAuthService(service *oauth.Service, sessions oauth.HubSessionReader, client oauth.Client, loginURL string, hubSessionTTL time.Duration) {
	s.hubSessionTTL = hubSessionTTL
	redirect, err := url.Parse(client.RedirectURI)
	if err != nil || !redirect.IsAbs() {
		return
	}
	s.oauth, s.hubSessions, s.oauthClient, s.oauthRedirect, s.oauthLoginURL = service, sessions, client, redirect, loginURL
}

// SetCurrentUserRepository is used by composition and focused profile tests.
func (s *Server) SetCurrentUserRepository(repository currentUserRepository) {
	s.currentUsers = repository
}

// SetRefreshService is used by composition and focused handler tests.
func (s *Server) SetRefreshService(service refresh.Refresher) { s.refresh = service }

// SetSessionService wires the RF-016 refresh-family manager.
func (s *Server) SetSessionService(service sessionManager) { s.sessions = service }

// SetLogoutService is used by composition and focused handler tests.
func (s *Server) SetLogoutService(service logout.Revoker) { s.logout = service }

// SetAdminUserService is used by composition and focused admin handler tests.
func (s *Server) SetAdminUserService(service admin.Manager) { s.adminUsers = service }

// SetRoleGridService is used by composition and focused role-grid handler tests.
func (s *Server) SetRoleGridService(service rolegrid.Manager) { s.roleGrid = service }

// SetAuditLogService is used by composition and focused audit-log handler tests.
func (s *Server) SetAuditLogService(service auditlog.Reader) { s.auditLog = service }

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

	return s.oauthCORS(HandlerWithOptions(s, ChiServerOptions{BaseRouter: r, ErrorHandlerFunc: s.handleBindingError}))
}

// handleBindingError overrides oapi-codegen's default binding error response
// (a plain 400) for the refresh_token cookie parameter: a missing or
// malformed cookie on /auth/refresh and /auth/logout must behave exactly
// like an invalid token (401, with the compromised cookie cleared), per
// RF-005/RF-007, not surface as a generic bad request.
func (s *Server) handleBindingError(w http.ResponseWriter, r *http.Request, err error) {
	if r.URL.Path == "/oauth/authorize" && s.redirectOAuthError(w, r) {
		return
	}
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
	writeProblem(w, http.StatusBadRequest, "invalid-request", titleBadRequest, "The request could not be processed.")
}

func (s *Server) ListAuditLog(w http.ResponseWriter, r *http.Request, params ListAuditLogParams) {
	RequireAuth(s.tokens)(RequireRole(s.currentUsers, "admin")(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		s.listAuditLog(w, request, params)
	}))).ServeHTTP(w, r)
}
func (s *Server) ListUsers(w http.ResponseWriter, r *http.Request, params ListUsersParams) {
	RequireAuth(s.tokens)(RequireRole(s.currentUsers, "admin")(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		s.listUsers(w, request, params)
	}))).ServeHTTP(w, r)
}
func (s *Server) GetUser(w http.ResponseWriter, r *http.Request, userID UserId) {
	RequireAuth(s.tokens)(RequireRole(s.currentUsers, "admin")(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		s.getUser(w, request, userID)
	}))).ServeHTTP(w, r)
}
func (s *Server) UpdateUser(w http.ResponseWriter, r *http.Request, userID UserId) {
	RequireAuth(s.tokens)(RequireRole(s.currentUsers, "admin")(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		s.updateUser(w, request, userID)
	}))).ServeHTTP(w, r)
}
func (s *Server) CreateEmployee(w http.ResponseWriter, r *http.Request) {
	RequireAuth(s.tokens)(RequireRole(s.currentUsers, "admin")(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		s.createEmployee(w, request)
	}))).ServeHTTP(w, r)
}
func (s *Server) ResendInvitation(w http.ResponseWriter, r *http.Request, userID UserId) {
	RequireAuth(s.tokens)(RequireRole(s.currentUsers, "admin")(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		s.resendInvitation(w, request, userID)
	}))).ServeHTTP(w, r)
}

// adminOnly wraps a handler with authentication and the admin role, like the other admin routes.
func (s *Server) adminOnly(next http.HandlerFunc) http.Handler {
	return RequireAuth(s.tokens)(RequireRole(s.currentUsers, "admin")(next))
}

func (s *Server) ListApplications(w http.ResponseWriter, r *http.Request) {
	s.adminOnly(s.listApplications).ServeHTTP(w, r)
}

func (s *Server) ListApplicationRoles(w http.ResponseWriter, r *http.Request, applicationID ApplicationId) {
	s.adminOnly(func(w http.ResponseWriter, r *http.Request) { s.listApplicationRoles(w, r, applicationID) }).ServeHTTP(w, r)
}

func (s *Server) CreateApplicationRole(w http.ResponseWriter, r *http.Request, applicationID ApplicationId) {
	s.adminOnly(func(w http.ResponseWriter, r *http.Request) { s.createApplicationRole(w, r, applicationID) }).ServeHTTP(w, r)
}

func (s *Server) UpdateApplicationRole(w http.ResponseWriter, r *http.Request, applicationID ApplicationId, roleID RoleId) {
	s.adminOnly(func(w http.ResponseWriter, r *http.Request) { s.updateApplicationRole(w, r, applicationID, roleID) }).ServeHTTP(w, r)
}

func (s *Server) DeleteApplicationRole(w http.ResponseWriter, r *http.Request, applicationID ApplicationId, roleID RoleId) {
	s.adminOnly(func(w http.ResponseWriter, r *http.Request) { s.deleteApplicationRole(w, r, applicationID, roleID) }).ServeHTTP(w, r)
}

// AcceptInvitation is unauthenticated (security: [] in openapi.yaml): the
// invitation token itself, not a bearer token, proves the caller may set
// this account's password (RF-002).
func (s *Server) AcceptInvitation(w http.ResponseWriter, r *http.Request) {
	s.acceptInvitation(w, r)
}
func (s *Server) ResendMfaCode(w http.ResponseWriter, r *http.Request) { s.resendMfaCode(w, r) }
func (s *Server) AuthorizeClient(w http.ResponseWriter, r *http.Request, params AuthorizeClientParams) {
	s.authorizeClient(w, r, params)
}
func (s *Server) ExchangeAuthorizationCode(w http.ResponseWriter, r *http.Request) {
	s.exchangeAuthorizationCode(w, r)
}
func (s *Server) VerifyMfa(w http.ResponseWriter, r *http.Request) { s.verifyMfa(w, r) }
func (s *Server) ConfirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	s.confirmPasswordReset(w, r)
}
func (s *Server) RequestPasswordReset(w http.ResponseWriter, r *http.Request) {
	s.requestPasswordReset(w, r)
}

func (s *Server) GetHealth(w http.ResponseWriter, r *http.Request)    { s.Health(w, r) }
func (s *Server) GetReadiness(w http.ResponseWriter, r *http.Request) { s.Readiness(w, r) }
