// Package oauth implements the Authorization Code + PKCE flow for the public
// clients registered in the applications table (ADR 0009, ADR 0015).
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

const AuthorizationCodeTTL = time.Minute

const (
	PKCEVerifierMinLength  = 43
	PKCEVerifierMaxLength  = 128
	PKCEChallengeMinLength = 43
	PKCEChallengeMaxLength = 128
)

var (
	ErrInvalidClient            = errors.New("oauth client is invalid")
	ErrInvalidRequest           = errors.New("oauth request is invalid")
	ErrAuthorizationCodeInvalid = errors.New("authorization code is invalid")
	// ErrHubSessionInvalid intentionally does not share the authorization-code
	// sentinel: callers redirect to login only for an absent, expired, revoked,
	// or inactive Hub session; database failures remain server errors.
	ErrHubSessionInvalid = errors.New("hub session is invalid")
	// ErrAuthorizationCodeReused is returned by the repository, together with the
	// code owner in StoredCode.UserID, when the hash matches an already used code.
	ErrAuthorizationCodeReused = errors.New("authorization code was already used")
)

// Client is one registered application: its client_id (also the token
// audience), exact redirect URI, allowed CORS origin and owning company.
type Client struct {
	ID, RedirectURI, Origin string
	OrganizationID          uuid.UUID
}

// Reasons recorded when /oauth/authorize refuses an authenticated user
// (ADR 0015 point 4).
const (
	RejectionOrganizationMismatch = "organization_mismatch"
	RejectionNoApplicationRole    = "no_application_role"
	// AuditActionAuthorizationRejected is the audit action of a refused authorization.
	AuditActionAuthorizationRejected = "oauth_authorization_rejected"
)

// RejectedError reports that the user may not use the application. No code was issued.
type RejectedError struct{ Reason string }

func (e *RejectedError) Error() string { return "oauth authorization rejected: " + e.Reason }

// AuthorizationAccess is what the authorization decision needs to know about a
// user and an application.
type AuthorizationAccess struct {
	UserOrganizationID, ApplicationOrganizationID uuid.UUID
	HasApplicationRole                            bool
}
type AuthorizeInput struct{ ClientID, RedirectURI, ResponseType, State, CodeChallenge, CodeChallengeMethod string }
type ExchangeInput struct{ Code, ClientID, RedirectURI, CodeVerifier string }
type AuthorizeResult struct {
	Code      string
	ExpiresAt time.Time
}
type ExchangeResult struct {
	UserID      uuid.UUID
	ClientID    string
	Roles       []string
	Permissions []string
}
type CreateCode struct {
	ID, UserID            uuid.UUID
	ClientID, RedirectURI string
	CodeHash              []byte
	CodeChallenge         string
	ExpiresAt             time.Time
}
type StoredCode struct {
	ID, UserID            uuid.UUID
	ClientID, RedirectURI string
	CodeHash              []byte
	CodeChallenge         string
	ExpiresAt, UsedAt     time.Time
}
type AuditEvent struct {
	ActorUserID uuid.UUID
	Action      string
	Metadata    map[string]string
}

type Repository interface {
	CreateAuthorizationCode(context.Context, CreateCode) error
	InsertAuditEvent(context.Context, AuditEvent) error
	GetAuthorizationAccess(ctx context.Context, userID uuid.UUID, clientID string) (AuthorizationAccess, error)
	WithinAuthorizationCodeTransaction(context.Context, func(ExchangeWriter) error) error
}
type ExchangeWriter interface {
	ExchangeAuthorizationCode(context.Context, []byte, string, string, string, time.Time) (StoredCode, error)
	// ListApplicationRolesForUser returns only the roles whose application is clientID.
	ListApplicationRolesForUser(ctx context.Context, userID uuid.UUID, clientID string) ([]string, error)
	ListPermissionKeysForRolesAndApplication(context.Context, []string, string) ([]string, error)
	InsertAuditEvent(context.Context, AuditEvent) error
}
type HubSessionReader interface {
	GetHubSessionUser(context.Context, []byte) (uuid.UUID, error)
}

type Service struct {
	repository Repository
	clients    map[string]Client
	origins    map[string]struct{}
	random     io.Reader
	now        func() time.Time
}

// New builds the service over the registered clients; ValidateClients checks them at startup.
func New(repository Repository, clients []Client, random io.Reader, now func() time.Time) *Service {
	if random == nil {
		random = rand.Reader
	}
	if now == nil {
		now = time.Now
	}
	byID := make(map[string]Client, len(clients))
	origins := make(map[string]struct{}, len(clients))
	for _, client := range clients {
		byID[client.ID] = client
		origins[client.Origin] = struct{}{}
	}
	return &Service{repository: repository, clients: byID, origins: origins, random: random, now: now}
}

func (s *Service) Authorize(ctx context.Context, userID uuid.UUID, input AuthorizeInput) (AuthorizeResult, error) {
	if err := s.ValidateAuthorizeInput(input); err != nil {
		return AuthorizeResult{}, err
	}
	client := s.clients[input.ClientID]
	if err := s.checkAccess(ctx, userID, client); err != nil {
		return AuthorizeResult{}, err
	}
	raw := make([]byte, 32)
	if _, err := io.ReadFull(s.random, raw); err != nil {
		return AuthorizeResult{}, fmt.Errorf("generate authorization code: %w", err)
	}
	now := s.now().UTC()
	expiresAt := now.Add(AuthorizationCodeTTL)
	sum := sha256.Sum256(raw)
	if err := s.repository.CreateAuthorizationCode(ctx, CreateCode{ID: uuid.New(), UserID: userID, ClientID: client.ID, RedirectURI: client.RedirectURI, CodeHash: sum[:], CodeChallenge: input.CodeChallenge, ExpiresAt: expiresAt}); err != nil {
		return AuthorizeResult{}, fmt.Errorf("create authorization code: %w", err)
	}
	if err := s.repository.InsertAuditEvent(ctx, AuditEvent{ActorUserID: userID, Action: "authorization_code_issued"}); err != nil {
		return AuthorizeResult{}, fmt.Errorf("audit authorization code issuance: %w", err)
	}
	return AuthorizeResult{Code: base64.RawURLEncoding.EncodeToString(raw), ExpiresAt: expiresAt}, nil
}

// checkAccess refuses, with an audit event and no code, a user of another
// company or without a role in the application (ADR 0015 point 4).
func (s *Service) checkAccess(ctx context.Context, userID uuid.UUID, client Client) error {
	access, err := s.repository.GetAuthorizationAccess(ctx, userID, client.ID)
	if err != nil {
		return fmt.Errorf("get authorization access: %w", err)
	}
	reason := ""
	switch {
	case access.UserOrganizationID != access.ApplicationOrganizationID:
		reason = RejectionOrganizationMismatch
	case !access.HasApplicationRole:
		reason = RejectionNoApplicationRole
	default:
		return nil
	}
	event := AuditEvent{ActorUserID: userID, Action: AuditActionAuthorizationRejected, Metadata: map[string]string{"reason": reason, "client_id": client.ID}}
	if err := s.repository.InsertAuditEvent(ctx, event); err != nil {
		return fmt.Errorf("audit rejected authorization: %w", err)
	}
	return &RejectedError{Reason: reason}
}

func (s *Service) Exchange(ctx context.Context, input ExchangeInput) (ExchangeResult, error) {
	if !s.validExchangeInput(input) {
		return ExchangeResult{}, ErrAuthorizationCodeInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(input.Code)
	if err != nil {
		return ExchangeResult{}, ErrAuthorizationCodeInvalid
	}
	hash := sha256.Sum256(raw)
	now := s.now().UTC()
	encoded := pkceS256Challenge(input.CodeVerifier)
	var outcome exchangeOutcome
	err = s.repository.WithinAuthorizationCodeTransaction(ctx, func(writer ExchangeWriter) error {
		var txErr error
		outcome, txErr = s.exchangeCode(ctx, writer, hash[:], input, encoded, now)
		return txErr
	})
	if err != nil {
		return ExchangeResult{}, err
	}
	if outcome.domainErr != nil {
		return ExchangeResult{}, outcome.domainErr
	}
	return outcome.result, nil
}

// exchangeOutcome carries what the exchange transaction decided: either a
// result or a domain error that must still commit the transaction.
type exchangeOutcome struct {
	result    ExchangeResult
	domainErr error
}

// validExchangeInput reports whether the request names the trusted client and
// redirect URI of a registered client and carries both a code and a PKCE verifier.
func (s *Service) validExchangeInput(input ExchangeInput) bool {
	client, ok := s.clients[input.ClientID]
	return ok && input.RedirectURI == client.RedirectURI && input.Code != "" && input.CodeVerifier != ""
}

// pkceS256Challenge derives the S256 PKCE challenge of a verifier.
func pkceS256Challenge(verifier string) string {
	challenge := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(challenge[:])
}

// exchangeCode runs inside the transaction: it consumes the code (auditing
// reuse) and, on success, builds the result for the user.
func (s *Service) exchangeCode(ctx context.Context, writer ExchangeWriter, hash []byte, input ExchangeInput, encoded string, now time.Time) (exchangeOutcome, error) {
	code, exchangeErr := writer.ExchangeAuthorizationCode(ctx, hash, input.ClientID, input.RedirectURI, encoded, now)
	switch {
	case errors.Is(exchangeErr, ErrAuthorizationCodeReused):
		if auditErr := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: code.UserID, Action: "authorization_code_reused"}); auditErr != nil {
			return exchangeOutcome{}, fmt.Errorf("audit reused authorization code: %w", auditErr)
		}
		return exchangeOutcome{domainErr: ErrAuthorizationCodeInvalid}, nil
	case errors.Is(exchangeErr, ErrAuthorizationCodeInvalid):
		return exchangeOutcome{domainErr: ErrAuthorizationCodeInvalid}, nil
	case exchangeErr != nil:
		return exchangeOutcome{}, fmt.Errorf("exchange authorization code: %w", exchangeErr)
	}
	result, err := s.exchangeResult(ctx, writer, code.UserID, input.ClientID)
	if err != nil {
		return exchangeOutcome{}, err
	}
	return exchangeOutcome{result: result}, nil
}

// exchangeResult looks up the user's roles and permissions in the requested
// application only and audits the successful exchange.
func (s *Service) exchangeResult(ctx context.Context, writer ExchangeWriter, userID uuid.UUID, clientID string) (ExchangeResult, error) {
	applicationRoles, rolesErr := writer.ListApplicationRolesForUser(ctx, userID, clientID)
	if rolesErr != nil {
		return ExchangeResult{}, fmt.Errorf("list application roles: %w", rolesErr)
	}
	if auditErr := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: userID, Action: "authorization_code_exchanged"}); auditErr != nil {
		return ExchangeResult{}, fmt.Errorf("audit authorization code exchange: %w", auditErr)
	}
	permissions, permissionsErr := writer.ListPermissionKeysForRolesAndApplication(ctx, applicationRoles, clientID)
	if permissionsErr != nil {
		return ExchangeResult{}, fmt.Errorf("list application permissions: %w", permissionsErr)
	}
	return ExchangeResult{UserID: userID, ClientID: clientID, Roles: applicationRoles, Permissions: permissions}, nil
}

// ValidateAuthorizeInput is the one validation source used by both the HTTP
// adapter and the service, preventing redirect and issuance rules drifting.
func (s *Service) ValidateAuthorizeInput(input AuthorizeInput) error {
	if !s.IsTrustedClient(input) {
		return ErrInvalidClient
	}
	if input.ResponseType != "code" || input.State == "" || !validPKCEChallenge(input.CodeChallenge) || input.CodeChallengeMethod != "S256" {
		return ErrInvalidRequest
	}
	return nil
}

func (s *Service) IsTrustedClient(input AuthorizeInput) bool {
	_, ok := s.TrustedClient(input)
	return ok
}

// TrustedClient returns the registered client whose client_id and exact
// redirect URI match the request.
func (s *Service) TrustedClient(input AuthorizeInput) (Client, bool) {
	client, ok := s.clients[input.ClientID]
	if !ok || input.RedirectURI != client.RedirectURI {
		return Client{}, false
	}
	return client, true
}

// AllowsOrigin reports whether origin is, exactly, the allowed origin of a registered client.
func (s *Service) AllowsOrigin(origin string) bool {
	if origin == "" {
		return false
	}
	_, ok := s.origins[origin]
	return ok
}

// ValidateClients makes a bad registration an explicit startup error instead of
// a later failure per request. At least one client must be registered.
func ValidateClients(clients []Client) error {
	if len(clients) == 0 {
		return errors.New("no OAuth client is registered in the applications table")
	}
	seen := make(map[string]struct{}, len(clients))
	for _, client := range clients {
		if err := validateClient(client); err != nil {
			return fmt.Errorf("OAuth client %q: %w", client.ID, err)
		}
		if _, dup := seen[client.ID]; dup {
			return fmt.Errorf("OAuth client %q is registered twice", client.ID)
		}
		seen[client.ID] = struct{}{}
	}
	return nil
}

func validateClient(client Client) error {
	if client.ID == "" || strings.ContainsAny(client.ID, " \t\r\n") {
		return errors.New("client_id must be non-empty and contain no whitespace")
	}
	redirect, err := url.Parse(client.RedirectURI)
	if err != nil || !redirect.IsAbs() || redirect.Host == "" || (redirect.Scheme != "http" && redirect.Scheme != "https") {
		return errors.New("redirect_uri must be an absolute http(s) URL")
	}
	if redirect.Fragment != "" {
		return errors.New("redirect_uri must not carry a fragment")
	}
	origin, err := url.Parse(client.Origin)
	if err != nil || origin.Host == "" || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || (origin.Scheme != "http" && origin.Scheme != "https") {
		return errors.New("allowed_origin must be only scheme and host")
	}
	if origin.Scheme != redirect.Scheme || origin.Host != redirect.Host {
		return errors.New("allowed_origin must match the origin of redirect_uri")
	}
	return nil
}

func validPKCEChallenge(value string) bool {
	return validPKCE(value, PKCEChallengeMinLength, PKCEChallengeMaxLength, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_")
}

func ValidPKCEVerifier(value string) bool {
	return validPKCE(value, PKCEVerifierMinLength, PKCEVerifierMaxLength, "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~")
}

func validPKCE(value string, minLength, maxLength int, alphabet string) bool {
	if len(value) < minLength || len(value) > maxLength {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune(alphabet, character) {
			return false
		}
	}
	return true
}
