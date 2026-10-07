// Package employee implements the admin-only account-creation-plus-invitation
// use case (T5, RF-001, D6/D9). D9 retired public self-registration; an
// employee account can only be created by an administrator, who never sets
// or learns its password. It uses the same Argon2id, event-publish, and
// hash-only-token patterns as the rest of the authentication core.
package employee

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
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

var (
	ErrEmailExists = errors.New("email already registered")
	ErrPublish     = errors.New("publish invitation event")
)

// invitationTTL is fixed at 24h (RF-002); T6 (RF-015, password reset) shares
// the same verification_tokens storage with an independent purpose and TTL.
const invitationTTL = 24 * time.Hour

// placeholderPasswordBytes only needs to satisfy the Argon2id hash the users
// table requires by construction; the admin never learns this value, and the
// invitee overwrites it when accepting the invitation (RF-002).
const placeholderPasswordBytes = 32

type Hasher interface {
	Hash(password string) (string, error)
}
type Publisher interface {
	Publish(context.Context, string, any) error
}
type Repository interface {
	WithinEmployeeCreationTransaction(context.Context, func(store.EmployeeCreationWriter) error) error
}

type Input struct {
	Email, DisplayName string
	Roles              []string
	ActorUserID        uuid.UUID
	IP                 *netip.Addr
	UserAgent          *string
}

type Result struct {
	ID                 uuid.UUID
	Email, DisplayName string
	Status             string
	Roles              []string
	CreatedAt          time.Time
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

// CreateEmployee validates the request, assigns the base "user" role plus any
// valid requested roles (RF-001, RF-009/D8), persists the account in
// pending_verification with a placeholder Argon2id password nobody knows,
// generates a one-time invitation token, audits the admin as actor, and
// publishes the invitation event after the transaction is ready to commit.
//
// Invariant 11 (an admin cannot assign roles to itself) does not apply here:
// the account being created is brand new, so its ID can never equal the
// actor's (D6/D8; the invariant is enforced for PATCH in internal/auth/admin).
func (s *Service) CreateEmployee(ctx context.Context, input Input) (Result, error) {
	finalRoles, err := validate(input)
	if err != nil {
		return Result{}, err
	}
	placeholder := make([]byte, placeholderPasswordBytes)
	if _, err := io.ReadFull(s.random, placeholder); err != nil {
		return Result{}, fmt.Errorf("generate placeholder password: %w", err)
	}
	passwordHash, err := s.hasher.Hash(base64.RawURLEncoding.EncodeToString(placeholder))
	if err != nil {
		return Result{}, fmt.Errorf("hash placeholder password: %w", err)
	}
	var result Result
	err = s.repository.WithinEmployeeCreationTransaction(ctx, func(writer store.EmployeeCreationWriter) error {
		var err error
		result, err = s.createWithinTransaction(ctx, writer, input, finalRoles, passwordHash)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

// createWithinTransaction persists the account, its roles, the invitation and
// the audit entry, and publishes the invitation event before commit.
func (s *Service) createWithinTransaction(ctx context.Context, writer store.EmployeeCreationWriter, input Input, finalRoles []string, passwordHash string) (Result, error) {
	found, validateErr := writer.ValidateRoleNames(ctx, finalRoles)
	if validateErr != nil {
		return Result{}, fmt.Errorf("validate requested roles: %w", validateErr)
	}
	if len(found) != len(finalRoles) {
		return Result{}, &InvalidInputError{Field: "roles", Detail: "contains an unknown role"}
	}
	user, err := writer.CreateUser(ctx, store.CreateUserParams{Email: input.Email, PasswordHash: passwordHash, DisplayName: input.DisplayName})
	if err != nil {
		return Result{}, mapCreateUserError(err)
	}
	for _, role := range finalRoles {
		if err := writer.AddUserRole(ctx, user.ID, role, input.ActorUserID); err != nil {
			return Result{}, fmt.Errorf("assign role %s: %w", role, err)
		}
	}
	event, err := s.issueInvitation(ctx, writer, input, user)
	if err != nil {
		return Result{}, err
	}
	if err := s.publisher.Publish(ctx, events.TypeUserInvited, event); err != nil {
		return Result{}, fmt.Errorf("%w: %w", ErrPublish, err)
	}
	return Result{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName, Status: user.Status, Roles: finalRoles, CreatedAt: user.CreatedAt}, nil
}

// mapCreateUserError turns a duplicate-email failure into ErrEmailExists and
// wraps any other account creation error.
func mapCreateUserError(err error) error {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) && pgErr.SQLState() == "23505" {
		return ErrEmailExists
	}
	if errors.Is(err, ErrEmailExists) {
		return ErrEmailExists
	}
	return fmt.Errorf("create employee account: %w", err)
}

// issueInvitation persists a one-time invitation token, audits the admin as
// actor and returns the user.invited event carrying the raw token.
func (s *Service) issueInvitation(ctx context.Context, writer store.EmployeeCreationWriter, input Input, user store.User) (events.UserInvited, error) {
	rawToken := make([]byte, 32)
	if _, err := io.ReadFull(s.random, rawToken); err != nil {
		return events.UserInvited{}, fmt.Errorf("generate invitation token: %w", err)
	}
	tokenHash := sha256.Sum256(rawToken)
	expiresAt := s.now().UTC().Add(invitationTTL)
	if err := writer.CreateInvitationToken(ctx, store.CreateInvitationTokenParams{UserID: user.ID, TokenHash: tokenHash[:], ExpiresAt: expiresAt}); err != nil {
		return events.UserInvited{}, fmt.Errorf("persist invitation token: %w", err)
	}
	resourceType, resourceID := "user", user.ID.String()
	if _, err := writer.InsertAuditEvent(ctx, store.InsertAuditEventParams{ActorUserID: &input.ActorUserID, Action: "employee_created", ResourceType: &resourceType, ResourceID: &resourceID, IP: input.IP, UserAgent: input.UserAgent, Metadata: []byte(`{}`)}); err != nil {
		return events.UserInvited{}, fmt.Errorf("record employee creation audit: %w", err)
	}
	event := events.UserInvited{Envelope: events.NewEnvelope(events.TypeUserInvited, "")}
	event.Data.UserID, event.Data.Email, event.Data.DisplayName, event.Data.InvitationToken, event.Data.ExpiresAt = user.ID, user.Email, user.DisplayName, base64.RawURLEncoding.EncodeToString(rawToken), expiresAt
	return event, nil
}

// validate checks the request and returns the deduplicated role set the
// account receives: role existence is checked in the database transaction, and
// "user" is always included even if the admin omitted it
// (RF-001: "toda cuenta recibe el rol base user").
func validate(input Input) ([]string, error) {
	if utf8.RuneCountInString(input.Email) == 0 || utf8.RuneCountInString(input.Email) > 254 {
		return nil, &InvalidInputError{Field: "email", Detail: "must be a valid email address"}
	}
	parsed, err := mail.ParseAddress(input.Email)
	if err != nil || parsed.Address != input.Email || !strings.Contains(input.Email, "@") {
		return nil, &InvalidInputError{Field: "email", Detail: "must be a valid email address"}
	}
	if utf8.RuneCountInString(input.DisplayName) < 1 || utf8.RuneCountInString(input.DisplayName) > 100 {
		return nil, &InvalidInputError{Field: "displayName", Detail: "must contain 1 to 100 characters"}
	}
	if len(input.Roles) == 0 {
		return nil, &InvalidInputError{Field: "roles", Detail: "must include at least one role"}
	}
	final := []string{"user"}
	for _, role := range input.Roles {
		if role != "user" && !hasRole(final, role) {
			final = append(final, role)
		}
	}
	return final, nil
}

// hasRole mirrors internal/auth/admin's own helper: AdminCreateUserRequest
// declares uniqueItems for roles, but nothing validates that at runtime
// (no OpenAPI request-validation middleware is wired), so a duplicate role
// in the request is deduplicated here instead of causing a duplicate
// user_roles primary-key violation.
func hasRole(list []string, role string) bool {
	for _, candidate := range list {
		if candidate == role {
			return true
		}
	}
	return false
}

// Creator is the narrow API boundary for admin-driven employee creation.
type Creator interface {
	CreateEmployee(context.Context, Input) (Result, error)
}
