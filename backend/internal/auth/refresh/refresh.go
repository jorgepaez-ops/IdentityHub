// Package refresh implements one-time refresh token rotation and reuse detection.
package refresh

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jorgepaez/identity-hub/internal/auth/roles"
	"github.com/jorgepaez/identity-hub/internal/auth/token"
	"github.com/jorgepaez/identity-hub/internal/observability"
)

var (
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrRefreshReuse        = errors.New("refresh token reuse detected")
)

type RotationStatus string

const (
	RotationSucceeded RotationStatus = "succeeded"
	RotationInvalid   RotationStatus = "invalid"
	RotationReused    RotationStatus = "reused"
)

// Rotation is the result of an atomic conditional database update. A repository
// must return RotationReused when the presented hash belongs to a rotated token.
type Rotation struct {
	Status   RotationStatus
	UserID   uuid.UUID
	FamilyID uuid.UUID
	ParentID uuid.UUID
	Roles    []string
}

type RefreshToken struct {
	UserID    uuid.UUID
	TokenHash []byte
	FamilyID  uuid.UUID
	ParentID  uuid.UUID
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

type SecurityEvent struct {
	Type         string
	UserID       uuid.UUID
	FamilyID     uuid.UUID
	RevokedCount int64
	IP           *netip.Addr
}

type Writer interface {
	RotateRefreshToken(context.Context, []byte, RefreshToken) (Rotation, error)
	RevokeRefreshFamily(context.Context, uuid.UUID) (int64, error)
	ListRolesForUser(context.Context, uuid.UUID) ([]string, error)
	InsertAuditEvent(context.Context, AuditEvent) error
}

type Repository interface {
	WithinRefreshTransaction(context.Context, func(Writer) error) error
}

// Refresher is implemented by Service and used by the api package for
// composition and focused handler tests, mirroring login.Authenticator.
type Refresher interface {
	Refresh(context.Context, Input) (Result, error)
}

type EventPublisher interface {
	PublishSecurityEvent(context.Context, SecurityEvent) error
}

type Input struct {
	RefreshToken string
	IP           *netip.Addr
	UserAgent    *string
}

type Result struct {
	AccessToken  string
	RefreshToken string
	TokenType    string
	ExpiresIn    int
}

type Service struct {
	repository Repository
	tokens     *token.Service
	publisher  EventPublisher
	refreshTTL time.Duration
	now        func() time.Time
}

func New(repository Repository, tokens *token.Service, refreshTTL time.Duration) *Service {
	return &Service{repository: repository, tokens: tokens, refreshTTL: refreshTTL, now: time.Now}
}

func (s *Service) WithEventPublisher(publisher EventPublisher) *Service {
	s.publisher = publisher
	return s
}

func (s *Service) Refresh(ctx context.Context, input Input) (Result, error) {
	if s.repository == nil || s.tokens == nil || s.refreshTTL <= 0 || input.RefreshToken == "" {
		return Result{}, ErrInvalidRefreshToken
	}

	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return Result{}, fmt.Errorf("generate refresh token: %w", err)
	}
	nextHash := sha256.Sum256(raw)
	var outcome refreshOutcome
	err := s.repository.WithinRefreshTransaction(ctx, func(writer Writer) error {
		var err error
		outcome, err = s.rotateWithinTransaction(ctx, writer, input, raw, nextHash)
		return err
	})
	if err != nil {
		return Result{}, err
	}
	return s.finishAfterCommit(ctx, outcome)
}

// refreshOutcome carries what the rotation transaction decided: the issued
// result, or a reuse event plus the error to return once it has committed.
type refreshOutcome struct {
	result     Result
	event      *SecurityEvent
	refreshErr error
}

// rotateWithinTransaction rotates the presented token and either issues the
// new session or, on reuse, revokes the whole family and audits it.
func (s *Service) rotateWithinTransaction(ctx context.Context, writer Writer, input Input, raw []byte, nextHash [sha256.Size]byte) (refreshOutcome, error) {
	presentedHash, err := hashRefreshToken(input.RefreshToken)
	if err != nil {
		return refreshOutcome{}, ErrInvalidRefreshToken
	}
	rotation, err := writer.RotateRefreshToken(ctx, presentedHash, RefreshToken{
		TokenHash: nextHash[:], IP: input.IP, UserAgent: input.UserAgent, ExpiresAt: s.now().Add(s.refreshTTL),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return refreshOutcome{}, ErrInvalidRefreshToken
		}
		return refreshOutcome{}, fmt.Errorf("rotate refresh token: %w", err)
	}
	switch rotation.Status {
	case RotationSucceeded:
		return s.issueRotatedSession(ctx, writer, rotation, raw)
	case RotationReused:
		return s.handleReuse(ctx, writer, rotation, input)
	default:
		return refreshOutcome{}, ErrInvalidRefreshToken
	}
}

// issueRotatedSession issues the access token for a successful rotation.
func (s *Service) issueRotatedSession(ctx context.Context, writer Writer, rotation Rotation, raw []byte) (refreshOutcome, error) {
	userRoles, err := writer.ListRolesForUser(ctx, rotation.UserID)
	if err != nil {
		return refreshOutcome{}, fmt.Errorf("list user roles: %w", err)
	}
	// The refresh cookie renews the Hub console's own directory-scoped
	// token (D8, RF-009): an application role must not reappear here.
	accessToken, err := s.tokens.IssueForSession(rotation.UserID.String(), roles.Directory(userRoles), rotation.FamilyID.String())
	if err != nil {
		return refreshOutcome{}, fmt.Errorf("issue access token: %w", err)
	}
	return refreshOutcome{result: Result{AccessToken: accessToken, RefreshToken: base64.RawURLEncoding.EncodeToString(raw), TokenType: "Bearer", ExpiresIn: token.AccessTokenExpiresIn}}, nil
}

// handleReuse revokes the whole family and audits the reuse of a rotated token.
func (s *Service) handleReuse(ctx context.Context, writer Writer, rotation Rotation, input Input) (refreshOutcome, error) {
	revokedCount, err := writer.RevokeRefreshFamily(ctx, rotation.FamilyID)
	if err != nil {
		return refreshOutcome{}, fmt.Errorf("revoke refresh family: %w", err)
	}
	if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: &rotation.UserID, Action: "refresh_reuse_detected", Reason: "rotated_token_presented", IP: input.IP, UserAgent: input.UserAgent}); err != nil {
		return refreshOutcome{}, fmt.Errorf("record refresh reuse audit: %w", err)
	}
	event := &SecurityEvent{Type: "security.refresh_reuse_detected", UserID: rotation.UserID, FamilyID: rotation.FamilyID, RevokedCount: revokedCount, IP: input.IP}
	// Returning an error here would roll back the revocation and audit.
	return refreshOutcome{event: event, refreshErr: ErrRefreshReuse}, nil
}

// finishAfterCommit counts and publishes a committed reuse event and maps the
// outcome to the caller's result.
func (s *Service) finishAfterCommit(ctx context.Context, outcome refreshOutcome) (Result, error) {
	if outcome.event != nil {
		// Counted only after the revocation committed, so a rolled-back or retried
		// transaction never inflates the dashboard.
		observability.RefreshReuseDetected.Inc()
	}
	if outcome.event != nil && s.publisher != nil {
		if err := s.publisher.PublishSecurityEvent(ctx, *outcome.event); err != nil {
			// The family was already revoked and audited inside the committed
			// transaction; a notification failure must not mask that reuse
			// was detected, or the client would keep an already-revoked
			// cookie and the caller would see a misleading 500 instead of 401.
			return Result{}, errors.Join(outcome.refreshErr, fmt.Errorf("publish refresh reuse event: %w", err))
		}
	}
	if outcome.refreshErr != nil {
		return Result{}, outcome.refreshErr
	}
	return outcome.result, nil
}

// hashRefreshToken decodes the base64url token carried in the cookie back to
// its raw bytes before hashing, matching how login.Service and this
// service's own rotation hash the raw bytes on the issuing side.
func hashRefreshToken(raw string) ([]byte, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(decoded)
	return hash[:], nil
}
