// Package registration implements the account-registration use case.
package registration

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"net/netip"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

var (
	ErrEmailExists = errors.New("email already registered")
	ErrPublish     = errors.New("publish registration event")
)

type Hasher interface {
	Hash(password string) (string, error)
}
type Publisher interface {
	Publish(context.Context, string, any) error
}
type Repository interface {
	WithinRegistrationTransaction(context.Context, func(store.RegistrationWriter) error) error
}

type Input struct {
	Email, Password, DisplayName string
	IP                           *netip.Addr
	UserAgent                    *string
}
type Result struct {
	ID            uuid.UUID
	Email, Status string
}
type InvalidInputError struct{ Field, Detail string }

func (e *InvalidInputError) Error() string { return e.Field + ": " + e.Detail }

type Service struct {
	repository Repository
	publisher  Publisher
	hasher     Hasher
	random     io.Reader
	now        func() time.Time
}

func New(repository Repository, publisher Publisher, hasher Hasher, random io.Reader, now func() time.Time) *Service {
	return &Service{repository: repository, publisher: publisher, hasher: hasher, random: random, now: now}
}

func (s *Service) Register(ctx context.Context, input Input) (Result, error) {
	if err := validate(input); err != nil {
		return Result{}, err
	}
	passwordHash, err := s.hasher.Hash(input.Password)
	if err != nil {
		return Result{}, fmt.Errorf("hash password: %w", err)
	}
	var result Result
	err = s.repository.WithinRegistrationTransaction(ctx, func(writer store.RegistrationWriter) error {
		user, err := writer.CreateUser(ctx, store.CreateUserParams{Email: input.Email, PasswordHash: passwordHash, DisplayName: input.DisplayName})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrEmailExists
			}
			var pgErr interface{ SQLState() string }
			if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
				return ErrEmailExists
			}
			if errors.Is(err, ErrEmailExists) {
				return ErrEmailExists
			}
			return fmt.Errorf("create account: %w", err)
		}
		rawToken := make([]byte, 32)
		if _, err := io.ReadFull(s.random, rawToken); err != nil {
			return fmt.Errorf("generate verification token: %w", err)
		}
		tokenHash := sha256.Sum256(rawToken)
		expiresAt := s.now().UTC().Add(24 * time.Hour)
		if err := writer.CreateVerificationToken(ctx, store.CreateVerificationTokenParams{UserID: user.ID, TokenHash: tokenHash[:], ExpiresAt: expiresAt}); err != nil {
			return fmt.Errorf("persist verification token: %w", err)
		}
		resourceType, resourceID := "user", user.ID.String()
		if _, err := writer.InsertAuditEvent(ctx, store.InsertAuditEventParams{ActorUserID: &user.ID, Action: "user_registered", ResourceType: &resourceType, ResourceID: &resourceID, IP: input.IP, UserAgent: input.UserAgent, Metadata: []byte(`{}`)}); err != nil {
			return fmt.Errorf("record registration audit: %w", err)
		}
		event := events.UserRegistered{Envelope: events.NewEnvelope(events.TypeUserRegistered, "")}
		event.Data.UserID, event.Data.Email, event.Data.DisplayName, event.Data.VerificationToken, event.Data.ExpiresAt = user.ID, user.Email, user.DisplayName, base64.RawURLEncoding.EncodeToString(rawToken), expiresAt
		if err := s.publisher.Publish(ctx, events.TypeUserRegistered, event); err != nil {
			return fmt.Errorf("%w: %w", ErrPublish, err)
		}
		result = Result{ID: user.ID, Email: user.Email, Status: user.Status}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func validate(input Input) error {
	if utf8.RuneCountInString(input.Password) < 12 || utf8.RuneCountInString(input.Password) > 128 {
		return &InvalidInputError{Field: "password", Detail: "must contain 12 to 128 characters"}
	}
	if utf8.RuneCountInString(input.Email) == 0 || utf8.RuneCountInString(input.Email) > 254 {
		return &InvalidInputError{Field: "email", Detail: "must be a valid email address"}
	}
	parsed, err := mail.ParseAddress(input.Email)
	if err != nil || parsed.Address != input.Email || !strings.Contains(input.Email, "@") {
		return &InvalidInputError{Field: "email", Detail: "must be a valid email address"}
	}
	if utf8.RuneCountInString(input.DisplayName) < 1 || utf8.RuneCountInString(input.DisplayName) > 100 {
		return &InvalidInputError{Field: "displayName", Detail: "must contain 1 to 100 characters"}
	}
	return nil
}

// Registrar is the narrow API boundary for account registration.
type Registrar interface {
	Register(context.Context, Input) (Result, error)
}
