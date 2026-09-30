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
	"time"

	"github.com/google/uuid"
	"github.com/jorgepaez/identity-hub/internal/auth/roles"
)

const AuthorizationCodeTTL = time.Minute

var (
	ErrInvalidClient            = errors.New("oauth client is invalid")
	ErrInvalidRequest           = errors.New("oauth request is invalid")
	ErrAuthorizationCodeInvalid = errors.New("authorization code is invalid")
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
	UserID uuid.UUID
	Roles  []string
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
	ExchangeAuthorizationCode(context.Context, []byte, string, string, string, time.Time) (StoredCode, error)
	ListRolesForUser(context.Context, uuid.UUID) ([]string, error)
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
	if input.ClientID != s.client.ID || input.RedirectURI != s.client.RedirectURI {
		return AuthorizeResult{}, ErrInvalidClient
	}
	if input.ResponseType != "code" || input.State == "" || input.CodeChallenge == "" || input.CodeChallengeMethod != "S256" {
		return AuthorizeResult{}, ErrInvalidRequest
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
	if input.ClientID != s.client.ID || input.RedirectURI != s.client.RedirectURI || input.Code == "" || input.CodeVerifier == "" {
		return ExchangeResult{}, ErrAuthorizationCodeInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(input.Code)
	if err != nil {
		return ExchangeResult{}, ErrAuthorizationCodeInvalid
	}
	hash := sha256.Sum256(raw)
	now := s.now().UTC()
	challenge := sha256.Sum256([]byte(input.CodeVerifier))
	encoded := base64.RawURLEncoding.EncodeToString(challenge[:])
	code, err := s.repository.ExchangeAuthorizationCode(ctx, hash[:], input.ClientID, input.RedirectURI, encoded, now)
	switch {
	case errors.Is(err, ErrAuthorizationCodeReused):
		if auditErr := s.repository.InsertAuditEvent(ctx, AuditEvent{ActorUserID: code.UserID, Action: "authorization_code_reused"}); auditErr != nil {
			return ExchangeResult{}, fmt.Errorf("audit reused authorization code: %w", auditErr)
		}
		return ExchangeResult{}, ErrAuthorizationCodeInvalid
	case errors.Is(err, ErrAuthorizationCodeInvalid):
		return ExchangeResult{}, ErrAuthorizationCodeInvalid
	case err != nil:
		return ExchangeResult{}, fmt.Errorf("exchange authorization code: %w", err)
	}
	userRoles, err := s.repository.ListRolesForUser(ctx, code.UserID)
	if err != nil {
		return ExchangeResult{}, fmt.Errorf("list application roles: %w", err)
	}
	if err := s.repository.InsertAuditEvent(ctx, AuditEvent{ActorUserID: code.UserID, Action: "authorization_code_exchanged"}); err != nil {
		return ExchangeResult{}, fmt.Errorf("audit authorization code exchange: %w", err)
	}
	return ExchangeResult{UserID: code.UserID, Roles: roles.ForApplication(userRoles, s.client.ID)}, nil
}
