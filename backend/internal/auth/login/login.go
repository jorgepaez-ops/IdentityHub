// Package login implements password authentication and refresh-session issuance.
package login

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/password"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrMFAUnavailable     = errors.New("multi-factor authentication is not supported")
)

const accessTokenExpiresIn = 900

type Status string

const StatusActive Status = "active"

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Status       Status
	MFAEnabled   bool
}

type Input struct {
	Email     string
	Password  string
	IP        *netip.Addr
	UserAgent *string
}

type Result struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int
}

type RefreshToken struct {
	UserID    uuid.UUID
	TokenHash []byte
	FamilyID  uuid.UUID
	IP        *netip.Addr
	UserAgent *string
	ExpiresAt time.Time
}

type AuditEvent struct {
	ActorUserID *uuid.UUID
	Action      string
	Reason      string
	IP          *netip.Addr
	UserAgent   *string
}

type Writer interface {
	GetLoginUserByEmail(context.Context, string) (User, error)
	ListRolesForUser(context.Context, uuid.UUID) ([]string, error)
	UpdateLoginSuccess(context.Context, uuid.UUID, string) error
	CreateRefreshToken(context.Context, RefreshToken) error
	InsertAuditEvent(context.Context, AuditEvent) error
}

type Repository interface {
	WithinLoginTransaction(context.Context, func(Writer) error) error
}

type Authenticator interface {
	Login(context.Context, Input) (Result, error)
}

type Service struct {
	repository Repository
	tokens     *token.Service
	refreshTTL time.Duration
	now        func() time.Time
}

func New(repository Repository, tokens *token.Service, refreshTTL time.Duration) *Service {
	return &Service{repository: repository, tokens: tokens, refreshTTL: refreshTTL, now: time.Now}
}

func (s *Service) Login(ctx context.Context, input Input) (Result, error) {
	if s.repository == nil || s.tokens == nil || s.refreshTTL <= 0 {
		return Result{}, fmt.Errorf("login service is unavailable")
	}
	var result Result
	var authenticationErr error
	err := s.repository.WithinLoginTransaction(ctx, func(writer Writer) error {
		user, err := writer.GetLoginUserByEmail(ctx, input.Email)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				if decoyErr := password.VerifyDecoy(input.Password); decoyErr != nil {
					return fmt.Errorf("verify decoy password: %w", decoyErr)
				}
				if err := s.recordFailure(ctx, writer, nil, input, "invalid_credentials"); err != nil {
					return err
				}
				authenticationErr = ErrInvalidCredentials
				return nil
			}
			return fmt.Errorf("get login user: %w", err)
		}

		valid, err := password.Verify(input.Password, user.PasswordHash)
		if err != nil {
			return fmt.Errorf("verify password: %w", err)
		}
		if !valid || user.Status != StatusActive {
			if err := s.recordFailure(ctx, writer, &user.ID, input, "invalid_credentials"); err != nil {
				return err
			}
			authenticationErr = ErrInvalidCredentials
			return nil
		}
		if user.MFAEnabled {
			if err := s.recordFailure(ctx, writer, &user.ID, input, "mfa_not_supported"); err != nil {
				return err
			}
			authenticationErr = ErrMFAUnavailable
			return nil
		}

		updatedHash := user.PasswordHash
		if password.NeedsRehash(user.PasswordHash) {
			updatedHash, err = password.Hash(input.Password)
			if err != nil {
				return fmt.Errorf("rehash password: %w", err)
			}
		}
		roles, err := writer.ListRolesForUser(ctx, user.ID)
		if err != nil {
			return fmt.Errorf("list user roles: %w", err)
		}
		accessToken, err := s.tokens.Issue(user.ID.String(), roles)
		if err != nil {
			return fmt.Errorf("issue access token: %w", err)
		}
		refreshRaw := make([]byte, 32)
		if _, err := rand.Read(refreshRaw); err != nil {
			return fmt.Errorf("generate refresh token: %w", err)
		}
		refreshHash := sha256.Sum256(refreshRaw)
		if err := writer.UpdateLoginSuccess(ctx, user.ID, updatedHash); err != nil {
			return fmt.Errorf("update login success: %w", err)
		}
		if err := writer.CreateRefreshToken(ctx, RefreshToken{UserID: user.ID, TokenHash: refreshHash[:], FamilyID: uuid.New(), IP: input.IP, UserAgent: input.UserAgent, ExpiresAt: s.now().Add(s.refreshTTL)}); err != nil {
			return fmt.Errorf("create refresh token: %w", err)
		}
		if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: &user.ID, Action: "login_succeeded", IP: input.IP, UserAgent: input.UserAgent}); err != nil {
			return fmt.Errorf("record successful login audit: %w", err)
		}
		result = Result{AccessToken: accessToken, RefreshToken: string(refreshRaw), TokenType: "Bearer", ExpiresIn: accessTokenExpiresIn}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	if authenticationErr != nil {
		return Result{}, authenticationErr
	}
	return result, nil
}

func (s *Service) recordFailure(ctx context.Context, writer Writer, actorID *uuid.UUID, input Input, reason string) error {
	if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: actorID, Action: "login_failed", Reason: reason, IP: input.IP, UserAgent: input.UserAgent}); err != nil {
		return fmt.Errorf("record failed login audit: %w", err)
	}
	return nil
}
