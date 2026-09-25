// Package refresh implements one-time refresh token rotation and reuse detection.
package refresh

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
	"github.com/jorgepaez/identity-hub/internal/auth/token"
)

var (
	ErrInvalidRefreshToken = errors.New("invalid refresh token")
	ErrRefreshReuse        = errors.New("refresh token reuse detected")
)

const accessTokenExpiresIn = 900

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
	var result Result
	var event *SecurityEvent
	var refreshErr error
	err := s.repository.WithinRefreshTransaction(ctx, func(writer Writer) error {
		rotation, err := writer.RotateRefreshToken(ctx, hashRefreshToken(input.RefreshToken), RefreshToken{
			TokenHash: nextHash[:], IP: input.IP, UserAgent: input.UserAgent, ExpiresAt: s.now().Add(s.refreshTTL),
		})
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrInvalidRefreshToken
			}
			return fmt.Errorf("rotate refresh token: %w", err)
		}
		switch rotation.Status {
		case RotationSucceeded:
			roles, err := writer.ListRolesForUser(ctx, rotation.UserID)
			if err != nil {
				return fmt.Errorf("list user roles: %w", err)
			}
			accessToken, err := s.tokens.Issue(rotation.UserID.String(), roles)
			if err != nil {
				return fmt.Errorf("issue access token: %w", err)
			}
			result = Result{AccessToken: accessToken, RefreshToken: string(raw), TokenType: "Bearer", ExpiresIn: accessTokenExpiresIn}
			return nil
		case RotationReused:
			revokedCount, err := writer.RevokeRefreshFamily(ctx, rotation.FamilyID)
			if err != nil {
				return fmt.Errorf("revoke refresh family: %w", err)
			}
			if err := writer.InsertAuditEvent(ctx, AuditEvent{ActorUserID: &rotation.UserID, Action: "refresh_reuse_detected", Reason: "rotated_token_presented", IP: input.IP, UserAgent: input.UserAgent}); err != nil {
				return fmt.Errorf("record refresh reuse audit: %w", err)
			}
			event = &SecurityEvent{Type: "security.refresh_reuse_detected", UserID: rotation.UserID, FamilyID: rotation.FamilyID, RevokedCount: revokedCount, IP: input.IP}
			// Returning an error here would roll back the revocation and audit.
			refreshErr = ErrRefreshReuse
			return nil
		default:
			return ErrInvalidRefreshToken
		}
	})
	if err != nil {
		return Result{}, err
	}
	if event != nil && s.publisher != nil {
		if err := s.publisher.PublishSecurityEvent(ctx, *event); err != nil {
			// The family was already revoked and audited inside the committed
			// transaction; a notification failure must not mask that reuse
			// was detected, or the client would keep an already-revoked
			// cookie and the caller would see a misleading 500 instead of 401.
			return Result{}, errors.Join(refreshErr, fmt.Errorf("publish refresh reuse event: %w", err))
		}
	}
	if refreshErr != nil {
		return Result{}, refreshErr
	}
	return result, nil
}

func hashRefreshToken(raw string) []byte {
	hash := sha256.Sum256([]byte(raw))
	return hash[:]
}
