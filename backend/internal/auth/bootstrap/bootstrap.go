// Package bootstrap creates the optional first administrator during startup.
package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/roles"
	"github.com/jorgepaez/identity-hub/internal/events"
	"github.com/jorgepaez/identity-hub/internal/store"
)

const (
	invitationTTL           = 24 * time.Hour
	placeholderPasswordSize = 32
	invitationTokenSize     = 32
	// withdrawTimeout bounds the compensating transaction that runs after a
	// failed publish; it must not depend on the (possibly canceled) startup ctx.
	withdrawTimeout = 5 * time.Second
)

type safeOperationError struct {
	message string
	cause   error
}

func (e safeOperationError) Error() string { return e.message }
func (e safeOperationError) Unwrap() error { return e.cause }

type Outcome string

const (
	OutcomeDisabled                  Outcome = "disabled"
	OutcomeCreated                   Outcome = "created"
	OutcomeAdminExists               Outcome = "admin_exists"
	OutcomeEmailConflict             Outcome = "email_conflict"
	OutcomeInvitationReissued        Outcome = "invitation_reissued"
	OutcomeInvitationPending         Outcome = "invitation_pending"
	OutcomeInvitationUndelivered     Outcome = "invitation_undelivered"
	OutcomeInvitationUndeliveredLive Outcome = "invitation_undelivered_live"
)

type Hasher interface{ Hash(string) (string, error) }
type Publisher interface {
	Publish(context.Context, string, any) error
}
type Repository interface {
	WithinBootstrapAdminTransaction(context.Context, func(store.BootstrapAdminWriter) error) error
}

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

// Ensure creates an invitation-only administrator only when no administrator
// beyond a pending one exists (locked and disabled admins count), or replaces
// the invitation of a pending one. Database uncertainty fails startup:
// granting a privileged bootstrap account when that predicate cannot be
// decided is unsafe.
//
// user.invited carries the raw token, so it is published only after the
// transaction commits (like D16): never while holding the advisory lock, and
// never for a transaction that rolls back.
func (s *Service) Ensure(ctx context.Context, email string) (Outcome, error) {
	if strings.TrimSpace(email) == "" {
		return OutcomeDisabled, nil
	}
	if s.repository == nil || s.publisher == nil || s.hasher == nil || s.random == nil || s.now == nil {
		return "", errors.New("bootstrap administrator service is unavailable")
	}

	outcome := OutcomeDisabled
	var invitation *events.UserInvited
	err := s.repository.WithinBootstrapAdminTransaction(ctx, func(writer store.BootstrapAdminWriter) error {
		if err := writer.LockBootstrapAdmin(ctx); err != nil {
			return err
		}
		adminExists, err := writer.NonPendingAdminExists(ctx)
		if err != nil {
			return err
		}
		if adminExists {
			outcome = OutcomeAdminExists
			return nil
		}

		existing, err := writer.GetUserByEmail(ctx, email)
		if err == nil {
			// A pending bootstrap admin that never accepted its invitation would
			// leave the installation without any way in: replace the invitation.
			reissue, err := s.isPendingAdmin(ctx, writer, existing)
			if err != nil {
				return err
			}
			if !reissue {
				outcome = OutcomeEmailConflict
				return nil
			}
			live, err := writer.HasLiveInvitationToken(ctx, existing.ID)
			if err != nil {
				return safeOperationError{message: "bootstrap invitation lookup failed", cause: err}
			}
			if live {
				outcome = OutcomeInvitationPending
				return nil
			}
			if err := writer.InvalidateInvitationTokens(ctx, existing.ID); err != nil {
				return fmt.Errorf("invalidate old bootstrap invitation tokens: %w", err)
			}
			invitation, err = s.invite(ctx, writer, existing, "bootstrap_admin_invitation_reissued")
			if err != nil {
				return err
			}
			outcome = OutcomeInvitationReissued
			return nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return safeOperationError{message: "bootstrap account lookup failed", cause: err}
		}

		placeholder := make([]byte, placeholderPasswordSize)
		if _, err := io.ReadFull(s.random, placeholder); err != nil {
			return fmt.Errorf("generate bootstrap placeholder password: %w", err)
		}
		passwordHash, err := s.hasher.Hash(base64.RawURLEncoding.EncodeToString(placeholder))
		if err != nil {
			return fmt.Errorf("hash bootstrap placeholder password: %w", err)
		}
		user, err := writer.CreateUser(ctx, store.CreateUserParams{Email: email, PasswordHash: passwordHash, DisplayName: "Bootstrap Admin"})
		if err != nil {
			return safeOperationError{message: "bootstrap administrator creation failed", cause: err}
		}
		for _, role := range []string{roles.User, roles.Admin} {
			if err := writer.AddBootstrapUserRole(ctx, user.ID, role); err != nil {
				return fmt.Errorf("assign bootstrap role: %w", err)
			}
		}
		invitation, err = s.invite(ctx, writer, user, "bootstrap_admin_created")
		if err != nil {
			return err
		}
		outcome = OutcomeCreated
		return nil
	})
	if err != nil {
		return "", err
	}
	if invitation != nil {
		return s.publishAfterCommit(ctx, outcome, invitation), nil
	}
	return outcome, nil
}

// publishAfterCommit sends the committed invitation. When publishing fails it
// withdraws that token in a fresh short transaction so the next startup
// reissues at once instead of waiting for the 24 h expiry. Startup continues
// either way: the account is pending, has no password and cannot be used, so
// a broker outage is not a reason to keep the API down, and the failed
// publish is reported through the outcome, which main logs without the email
// or the token.
func (s *Service) publishAfterCommit(ctx context.Context, outcome Outcome, invitation *events.UserInvited) Outcome {
	if err := s.publisher.Publish(ctx, events.TypeUserInvited, *invitation); err == nil {
		return outcome
	}
	withdrawCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), withdrawTimeout)
	defer cancel()
	withdraw := s.repository.WithinBootstrapAdminTransaction(withdrawCtx, func(writer store.BootstrapAdminWriter) error {
		if err := writer.LockBootstrapAdmin(withdrawCtx); err != nil {
			return err
		}
		return writer.InvalidateInvitationTokens(withdrawCtx, invitation.Data.UserID)
	})
	if withdraw != nil {
		return OutcomeInvitationUndeliveredLive
	}
	return OutcomeInvitationUndelivered
}

func (s *Service) isPendingAdmin(ctx context.Context, writer store.BootstrapAdminWriter, user store.User) (bool, error) {
	if user.Status != "pending_verification" {
		return false, nil
	}
	names, err := writer.ListRolesForUser(ctx, user.ID)
	if err != nil {
		return false, safeOperationError{message: "bootstrap role lookup failed", cause: err}
	}
	return slices.Contains(names, roles.Admin), nil
}

// invite persists a fresh invitation token and records the system/bootstrap
// audit entry inside the transaction. It returns the user.invited event for
// the caller to publish once the transaction has committed.
func (s *Service) invite(ctx context.Context, writer store.BootstrapAdminWriter, user store.User, action string) (*events.UserInvited, error) {
	rawToken := make([]byte, invitationTokenSize)
	if _, err := io.ReadFull(s.random, rawToken); err != nil {
		return nil, fmt.Errorf("generate bootstrap invitation token: %w", err)
	}
	tokenHash := sha256.Sum256(rawToken)
	expiresAt := s.now().UTC().Add(invitationTTL)
	if err := writer.CreateInvitationToken(ctx, store.CreateInvitationTokenParams{UserID: user.ID, TokenHash: tokenHash[:], ExpiresAt: expiresAt}); err != nil {
		return nil, fmt.Errorf("persist bootstrap invitation token: %w", err)
	}
	resourceType, resourceID := "user", user.ID.String()
	if _, err := writer.InsertAuditEvent(ctx, store.InsertAuditEventParams{Action: action, ResourceType: &resourceType, ResourceID: &resourceID, Metadata: []byte(`{"actor":"system/bootstrap"}`)}); err != nil {
		return nil, fmt.Errorf("record bootstrap audit event: %w", err)
	}
	event := events.UserInvited{Envelope: events.NewEnvelope(events.TypeUserInvited, "")}
	event.Data.UserID, event.Data.Email, event.Data.DisplayName = user.ID, user.Email, user.DisplayName
	event.Data.InvitationToken, event.Data.ExpiresAt = base64.RawURLEncoding.EncodeToString(rawToken), expiresAt
	return &event, nil
}
