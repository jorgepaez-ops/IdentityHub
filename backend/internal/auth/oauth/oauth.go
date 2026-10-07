// Package oauth implements the narrow Authorization Code + PKCE flow used by
// the configured Contabilidad public client (ADR 0009).
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/roles"
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

type Client struct{ ID, RedirectURI, Origin string }
type AuthorizeInput struct{ ClientID, RedirectURI, ResponseType, State, CodeChallenge, CodeChallengeMethod string }
type ExchangeInput struct{ Code, ClientID, RedirectURI, CodeVerifier string }
type AuthorizeResult struct {
	Code      string
	ExpiresAt time.Time
}
type ExchangeResult struct {
	UserID      uuid.UUID
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
}

type Repository interface {
	CreateAuthorizationCode(context.Context, CreateCode) error
	InsertAuditEvent(context.Context, AuditEvent) error
	WithinAuthorizationCodeTransaction(context.Context, func(ExchangeWriter) error) error
}
type ExchangeWriter interface {
	ExchangeAuthorizationCode(context.Context, []byte, string, string, string, time.Time) (StoredCode, error)
	ListRolesForUser(context.Context, uuid.UUID) ([]string, error)
	ListPermissionKeysForRolesAndApplication(context.Context, []string, string) ([]string, error)
	InsertAuditEvent(context.Context, AuditEvent) error
}
type HubSessionReader interface {
	GetHubSessionUser(context.Context, []byte) (uuid.UUID, error)
}

type Service struct {
	repository Repository
	client     Client
	random     io.Reader
	now        func() time.Time
}

func New(repository Repository, client Client, random io.Reader, now func() time.Time) *Service {
	if random == nil {
		random = rand.Reader
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repository: repository, client: client, random: random, now: now}
}

func (s *Service) Authorize(ctx context.Context, userID uuid.UUID, input AuthorizeInput) (AuthorizeResult, error) {
	if err := s.ValidateAuthorizeInput(input); err != nil {
		return AuthorizeResult{}, err
	}
	raw := make([]byte, 32)
	if _, err := io.ReadFull(s.random, raw); err != nil {
		return AuthorizeResult{}, fmt.Errorf("generate authorization code: %w", err)
	}
	now := s.now().UTC()
	expiresAt := now.Add(AuthorizationCodeTTL)
	sum := sha256.Sum256(raw)
	if err := s.repository.CreateAuthorizationCode(ctx, CreateCode{ID: uuid.New(), UserID: userID, ClientID: s.client.ID, RedirectURI: s.client.RedirectURI, CodeHash: sum[:], CodeChallenge: input.CodeChallenge, ExpiresAt: expiresAt}); err != nil {
		return AuthorizeResult{}, fmt.Errorf("create authorization code: %w", err)
	}
	if err := s.repository.InsertAuditEvent(ctx, AuditEvent{ActorUserID: userID, Action: "authorization_code_issued"}); err != nil {
		return AuthorizeResult{}, fmt.Errorf("audit authorization code issuance: %w", err)
	}
	return AuthorizeResult{Code: base64.RawURLEncoding.EncodeToString(raw), ExpiresAt: expiresAt}, nil
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
// redirect URI and carries both a code and a PKCE verifier.
func (s *Service) validExchangeInput(input ExchangeInput) bool {
	return input.ClientID == s.client.ID && input.RedirectURI == s.client.RedirectURI && input.Code != "" && input.CodeVerifier != ""
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
	result, err := s.exchangeResult(ctx, writer, code.UserID)
	if err != nil {
		return exchangeOutcome{}, err
	}
	return exchangeOutcome{result: result}, nil
}

// exchangeResult looks up the user's application roles and permissions and
// audits the successful exchange.
func (s *Service) exchangeResult(ctx context.Context, writer ExchangeWriter, userID uuid.UUID) (ExchangeResult, error) {
	userRoles, rolesErr := writer.ListRolesForUser(ctx, userID)
	if rolesErr != nil {
		return ExchangeResult{}, fmt.Errorf("list application roles: %w", rolesErr)
	}
	if auditErr := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: userID, Action: "authorization_code_exchanged"}); auditErr != nil {
		return ExchangeResult{}, fmt.Errorf("audit authorization code exchange: %w", auditErr)
	}
	applicationRoles := roles.ForApplication(userRoles, s.client.ID)
	permissions, permissionsErr := writer.ListPermissionKeysForRolesAndApplication(ctx, applicationRoles, s.client.ID)
	if permissionsErr != nil {
		return ExchangeResult{}, fmt.Errorf("list application permissions: %w", permissionsErr)
	}
	return ExchangeResult{UserID: userID, Roles: applicationRoles, Permissions: permissions}, nil
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
	return input.ClientID == s.client.ID && input.RedirectURI == s.client.RedirectURI
}

// ValidateRegisteredClient prevents the fixed deployment configuration and
// seeded application registry from silently drifting apart at startup.
func ValidateRegisteredClient(configured, registered Client) error {
	if configured.ID != registered.ID || configured.RedirectURI != registered.RedirectURI || configured.Origin != registered.Origin {
		return fmt.Errorf("registered OAuth client does not match configured client")
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
